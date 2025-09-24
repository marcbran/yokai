package test

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-jsonnet"
	"github.com/marcbran/jsonnet-kit/pkg/jsonnext"
	"github.com/marcbran/yokai/internal/plugins/inout"
	"github.com/marcbran/yokai/internal/run"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	App run.AppConfig `mapstructure:"app"`
}

type Case struct {
	Name    string
	Inputs  []run.TopicPayload `json:"inputs"`
	Outputs []run.TopicPayload `json:"outputs"`
}

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
	var res Run
	var runErr error
	err := filepath.WalkDir(dirname, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !strings.HasSuffix(path, "_it.libsonnet") {
			return nil
		}
		r, err := RunFile(ctx, path)
		if err != nil {
			runErr = err
			_, err := os.Stderr.WriteString(err.Error())
			if err != nil {
				return err
			}
			return nil
		}
		if r != nil {
			res = res.append(path, *r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		return nil, errors.New("encountered at least one error while running tests")
	}
	return &res, nil
}

func RunFile(ctx context.Context, filename string) (*Run, error) {
	config := Config{
		App: run.AppConfig{
			Config: filename,
			Vendor: []string{},
		},
	}

	testCases, err := loadTestCases(filename)
	if err != nil {
		return nil, err
	}

	var res Run
	for _, testCase := range testCases {
		r, err := RunTestCase(ctx, &config, testCase)
		if err != nil {
			return nil, err
		}
		res = res.append(testCase.Name, *r)
	}
	return &res, nil
}

func RunTestCase(ctx context.Context, config *Config, testCase Case) (*Run, error) {
	registration := run.NewCompoundRegistration(
		[]run.Registration{
			run.NewAppRegistration(config.App),
			run.CommandRegistration{},
		},
	)

	inoutPlugin := inout.NewPlugin(testCase.Inputs)
	plugins := []run.Plugin{
		run.NewUpdaterPlugin(),
		inoutPlugin,
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, gCtx := errgroup.WithContext(runCtx)

	g.Go(func() error {
		return run.Run(gCtx, registration, plugins)
	})

	actualOutputs := inoutPlugin.Outputs()
	expectedOutputs := testCase.Outputs

	resultChan := make(chan bool, 1)

	g.Go(func() error {
		result := equalOutputs(expectedOutputs, actualOutputs)
		resultChan <- result
		return nil
	})

	timeout := time.After(5 * time.Second)

	select {
	case result := <-resultChan:
		cancel()
		err := g.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			return nil, err
		}
		if !result {
			return failedRun(), nil
		}
		return passedRun(), nil
	case <-timeout:
		cancel()
		return errorRun(errors.New("timeout waiting for outputs")), nil
	}
}

func equalOutputs(expected []run.TopicPayload, actualOutputs <-chan run.TopicPayload) bool {
	receivedOutputs := make([]run.TopicPayload, 0, len(expected))

	for len(receivedOutputs) < len(expected) {
		output, ok := <-actualOutputs
		if !ok {
			return false
		}
		receivedOutputs = append(receivedOutputs, output)
	}

	if len(expected) != len(receivedOutputs) {
		return false
	}

	for i, expectedOutput := range expected {
		actualOutput := receivedOutputs[i]
		if expectedOutput.Topic != actualOutput.Topic || expectedOutput.Payload != actualOutput.Payload {
			return false
		}
	}

	return true
}

//go:embed lib
var lib embed.FS

func loadTestCases(config string) ([]Case, error) {
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
	var testCases []Case
	err = json.Unmarshal([]byte(jsonStr), &testCases)
	if err != nil {
		return nil, err
	}
	return testCases, nil
}

func errorRun(err error) *Run {
	return &Run{
		Results: []Result{
			{
				Equal: false,
				Error: err.Error(),
			},
		},
		PassedCount: 0,
		TotalCount:  1,
	}
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

func failedRun() *Run {
	return &Run{
		Results: []Result{
			{
				Equal: false,
			},
		},
		PassedCount: 0,
		TotalCount:  1,
	}
}
