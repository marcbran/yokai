package it

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/compose/v2/pkg/progress"
	"github.com/google/go-jsonnet"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/cmd/formatter"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/marcbran/jsonnet-kit/pkg/jsonnext"
)

type Run struct {
	Results     []Result `json:"results"`
	PassedCount int      `json:"passedCount"`
	TotalCount  int      `json:"totalCount"`
}

func (r Run) append(prefix string, o Run) Run {
	var other []Result
	for _, result := range o.Results {
		other = append(other, Result{
			Name:  fmt.Sprintf("%s%s", prefix, result.Name),
			Equal: result.Equal,
			Error: result.Error,
		})
	}
	return Run{
		Results:     append(r.Results, other...),
		PassedCount: r.PassedCount + o.PassedCount,
		TotalCount:  r.TotalCount + o.TotalCount,
	}
}

type Result struct {
	Name  string `json:"name"`
	Equal bool   `json:"equal"`
	Error string `json:"error"`
}

func RunDir(ctx context.Context, dirname string) (*Run, error) {
	var run Run
	var runErr error
	err := filepath.WalkDir(dirname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !strings.HasSuffix(path, "_it.libsonnet") {
			return nil
		}
		r, err := RunFile(ctx, dirname, path)
		if err != nil {
			runErr = err
			_, err := os.Stderr.WriteString(err.Error())
			if err != nil {
				return err
			}
			return nil
		}
		if r != nil {
			run = run.append(path, *r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		return nil, errors.New("encountered at least one error while running tests")
	}
	return &run, nil
}

//go:embed lib
var lib embed.FS

func RunFile(ctx context.Context, dirname, filename string) (*Run, error) {
	testCases, err := loadTestCases(filename)
	if err != nil {
		return nil, err
	}
	dockerCompose, err := renderDockerCompose(dirname, filename, "dev-arm64v8")
	if err != nil {
		return nil, err
	}

	var res Run
	for _, testCase := range testCases {
		run, err := RunTestCase(ctx, testCase, dockerCompose)
		if err != nil {
			return nil, err
		}
		res = res.append(testCase.Name, *run)
	}
	return &res, nil
}

func RunTestCase(ctx context.Context, testCase TestCase, dockerCompose map[string]any) (*Run, error) {
	composeService, err := newComposeService()
	if err != nil {
		return nil, err
	}

	project, err := createProject(ctx, dockerCompose)
	if err != nil {
		return nil, err
	}

	cleanup, err := runProject(ctx, composeService, project)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := cleanup()
		if err != nil {
			log.WithError(err).Warn("cleanup failed")
		}
	}()

	time.Sleep(time.Second)

	err = publishInputs(ctx, composeService, project, testCase)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Second)

	actualOutputs, err := consumeOutputs(ctx, composeService, project)
	if err != nil {
		return nil, err
	}

	passed := equalOutputs(testCase.Outputs, actualOutputs)

	if !passed {
		return failedRun(ctx, composeService, project)
	}
	return passedRun(), nil
}

func newComposeService() (api.Service, error) {
	cli, err := command.NewDockerCli()
	if err != nil {
		return nil, err
	}
	clientOpts := &flags.ClientOptions{}
	err = cli.Initialize(clientOpts, command.WithErrorStream(io.Discard))
	if err != nil {
		return nil, err
	}
	composeService := compose.NewComposeService(cli)
	return composeService, nil
}

func createProject(ctx context.Context, dockerCompose map[string]any) (*types.Project, error) {
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Config: dockerCompose,
			},
		},
	}

	project, err := loader.LoadWithContext(ctx, configDetails, func(opts *loader.Options) {
		opts.SkipResolveEnvironment = true
		opts.SetProjectName(fmt.Sprintf("yokai-it-%s", uuid.NewString()), true)
	})
	if err != nil {
		return nil, err
	}
	project = project.WithoutUnnecessaryResources()

	for i, s := range project.Services {
		s.CustomLabels = map[string]string{
			api.ProjectLabel: project.Name,
			api.ServiceLabel: s.Name,
			api.VersionLabel: api.ComposeVersion,
			api.OneoffLabel:  "False",
		}
		project.Services[i] = s
	}
	return project, nil
}

func runProject(ctx context.Context, composeService api.Service, project *types.Project) (func() error, error) {
	progress.Mode = progress.ModeQuiet

	err := composeService.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{
			Recreate: api.RecreateDiverged,
		},
		Start: api.StartOptions{
			Wait: true,
		},
	})
	if err != nil {
		return nil, err
	}
	return func() error {
		err := composeService.Down(ctx, project.Name, api.DownOptions{
			Volumes:       true,
			RemoveOrphans: true,
		})
		if err != nil {
			return err
		}
		err = composeService.Remove(ctx, project.Name, api.RemoveOptions{
			Volumes: true,
			Force:   true,
		})
		if err != nil {
			return err
		}
		return nil
	}, nil
}

type TestCase struct {
	Name    string
	Inputs  []TopicPayload `json:"inputs"`
	Outputs []TopicPayload `json:"outputs"`
}

type TopicPayload struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
}

func loadTestCases(config string) ([]TestCase, error) {
	vm := jsonnet.MakeVM()
	vm.Importer(jsonnext.CompoundImporter{
		Importers: []jsonnet.Importer{
			&jsonnext.FSImporter{Fs: lib},
			&jsonnet.FileImporter{},
		},
	})
	vm.TLACode("testConfig", fmt.Sprintf("import '%s'", config))
	jsonStr, err := vm.EvaluateFile("./lib/load_test_cases.libsonnet")
	if err != nil {
		return nil, err
	}
	var testCases []TestCase
	err = json.Unmarshal([]byte(jsonStr), &testCases)
	if err != nil {
		return nil, err
	}
	return testCases, nil
}

func renderDockerCompose(rootDir, configFile, version string) (map[string]any, error) {
	vm := jsonnet.MakeVM()
	vm.Importer(jsonnext.CompoundImporter{
		Importers: []jsonnet.Importer{
			&jsonnext.FSImporter{Fs: lib},
			&jsonnet.FileImporter{},
		},
	})
	vm.TLAVar("rootDir", rootDir)
	vm.TLAVar("configFile", configFile)
	vm.TLAVar("version", version)
	jsonStr, err := vm.EvaluateFile("./lib/render_docker_compose.libsonnet")
	if err != nil {
		return nil, err
	}
	var dockerCompose map[string]any
	err = json.Unmarshal([]byte(jsonStr), &dockerCompose)
	if err != nil {
		return nil, err
	}
	return dockerCompose, nil
}

func publishInputs(ctx context.Context, composeService api.Service, project *types.Project, testCase TestCase) error {
	for _, input := range testCase.Inputs {
		_, err := composeService.RunOneOffContainer(ctx, project, api.RunOptions{
			Service: "cli",
			Command: []string{"pub", "-h", "mqtt", "-t", input.Topic, "-m", input.Payload},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func consumeOutputs(ctx context.Context, composeService api.Service, project *types.Project) ([]TopicPayload, error) {
	topicConsumer := newTopicConsumer()
	err := composeService.Logs(ctx, project.Name, topicConsumer, api.LogOptions{
		Follow:   false,
		Services: []string{"cli"},
	})
	if err != nil {
		return nil, err
	}
	return topicConsumer.data, nil
}

type topicConsumer struct {
	data []TopicPayload
}

func newTopicConsumer() *topicConsumer {
	return &topicConsumer{}
}

func (t *topicConsumer) Log(containerName, message string) {
	parts := strings.SplitN(message, ": ", 2)
	if len(parts) != 2 {
		return
	}
	topic, payload := parts[0], parts[1]
	t.data = append(t.data, TopicPayload{
		Topic:   topic,
		Payload: payload,
	})
}

func (t *topicConsumer) Err(containerName, message string) {
}

func (t *topicConsumer) Status(container, msg string) {
}

func (t *topicConsumer) Register(container string) {
}

func equalOutputs(expectedOutputs []TopicPayload, actualOutputs []TopicPayload) bool {
	expectedIndex := 0
	actualIndex := 0
	passed := true
	for {
		if expectedIndex >= len(expectedOutputs) {
			passed = true
			break
		}
		if actualIndex >= len(actualOutputs) {
			passed = false
			break
		}

		actual := actualOutputs[actualIndex]
		expected := expectedOutputs[expectedIndex]

		if actual.Topic != expected.Topic {
			actualIndex++
			continue
		}
		if actual.Payload != expected.Payload {
			passed = false
			break
		}
		actualIndex++
		expectedIndex++
	}
	return passed
}

func failedRun(ctx context.Context, composeService api.Service, project *types.Project) (*Run, error) {
	var buf bytes.Buffer
	err := composeService.Logs(ctx, project.Name, formatter.NewLogConsumer(ctx, &buf, &buf, false, false, false), api.LogOptions{
		Follow: false,
	})
	if err != nil {
		return nil, err
	}
	return &Run{
		Results: []Result{
			{
				Equal: false,
				Error: buf.String(),
			},
		},
		PassedCount: 0,
		TotalCount:  1,
	}, nil
}

func passedRun() *Run {
	return &Run{
		Results: []Result{
			{
				Equal: true,
			},
		},
		PassedCount: 1,
		TotalCount:  1,
	}
}
