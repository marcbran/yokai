package run

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/marcbran/jsonnet-kit/pkg/jsonnext"

	"github.com/google/go-jsonnet"
)

type AppRegistration struct {
	appLib AppLib
}

func (a AppRegistration) Register() (Registry, error) {
	apps, err := a.appLib.listApps()
	if err != nil {
		return Registry{}, err
	}

	var models sync.Map
	res := NewRegistry()
	for key, app := range apps {
		models.Store(key, app.Init)
		model := &AppModel{
			key:         key,
			models:      &models,
			appLib:      a.appLib,
			subscribers: sync.Map{},
		}
		for _, topic := range app.Subscriptions {
			res.TopicToModels[topic] = append(res.TopicToModels[topic], model)
		}
		res.KeyToModel[key] = model
	}

	return res, nil
}

type AppModel struct {
	key         Key
	models      *sync.Map
	appLib      AppLib
	subscribers sync.Map
}

func (a *AppModel) Update(ctx context.Context, topic Topic, payload Payload) (map[Topic]Payload, error) {
	model, ok := a.models.Load(a.key)
	if !ok {
		return nil, fmt.Errorf("model not found for app %s", a.key)
	}

	updates, err := a.appLib.update(a.key, topic, payload, model)
	if err != nil {
		return nil, err
	}

	if model, ok := updates["model"]; ok {
		a.models.Store(a.key, model)

		view, err := a.appLib.view(a.key, model, true)
		if err != nil {
			return nil, err
		}

		a.subscribers.Range(func(ch, _ any) bool {
			select {
			case ch.(chan string) <- view:
			default:
				// Drop update if channel is full
			}
			return true
		})
	}

	outputs := make(map[Topic]Payload)
	for topic, payload := range updates {
		if topic == "model" {
			continue
		}

		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		outputs[topic] = string(b)
	}
	return outputs, nil
}

func (a *AppModel) View(ctx context.Context) (string, error) {
	model, ok := a.models.Load(a.key)
	if !ok {
		return "", fmt.Errorf("model not found for app %s", a.key)
	}

	view, err := a.appLib.view(a.key, model, false)
	if err != nil {
		return "", err
	}
	return view, nil
}

func (a *AppModel) SubscribeView() (<-chan string, func()) {
	ch := make(chan string, 100)

	a.subscribers.Store(ch, struct{}{})

	unsubscribe := func() {
		a.subscribers.Delete(ch)
		close(ch)
	}

	return ch, unsubscribe
}

type AppLib struct {
	config string
	vendor []string
}

type AppData struct {
	Init          any      `json:"init"`
	Subscriptions []string `json:"subscriptions"`
}

//go:embed lib
var lib embed.FS

func (a AppLib) vm() *jsonnet.VM {
	vm := jsonnet.MakeVM()
	vm.Importer(jsonnext.CompoundImporter{
		Importers: []jsonnet.Importer{
			&jsonnext.FSImporter{Fs: lib},
			&jsonnet.FileImporter{JPaths: a.vendor},
		},
	})
	return vm
}

func (a AppLib) listApps() (map[string]AppData, error) {
	vm := a.vm()
	vm.TLACode("config", fmt.Sprintf("import '%s'", a.config))
	jsonStr, err := vm.EvaluateFile("./lib/list_apps.libsonnet")
	if err != nil {
		return nil, err
	}
	var apps map[string]AppData
	err = json.Unmarshal([]byte(jsonStr), &apps)
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (a AppLib) update(key Key, topic Topic, payload Payload, model any) (map[string]any, error) {
	vm := a.vm()
	vm.TLACode("config", fmt.Sprintf("import '%s'", a.config))
	vm.TLAVar("key", key)
	vm.TLAVar("topic", topic)
	vm.TLACode("payload", payload)
	jsonModel, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	vm.TLACode("model", string(jsonModel))
	jsonStr, err := vm.EvaluateFile("./lib/update.libsonnet")
	if err != nil {
		return nil, err
	}
	var update map[string]any
	err = json.Unmarshal([]byte(jsonStr), &update)
	if err != nil {
		return nil, err
	}
	return update, nil
}

func (a AppLib) view(key Key, model any, fragment bool) (string, error) {
	vm := a.vm()
	vm.TLACode("config", fmt.Sprintf("import '%s'", a.config))
	vm.TLAVar("key", key)
	vm.TLACode("fragment", fmt.Sprintf("%t", fragment))
	jsonModel, err := json.Marshal(model)
	if err != nil {
		return "", err
	}
	vm.TLACode("model", string(jsonModel))
	jsonStr, err := vm.EvaluateFile("./lib/view.libsonnet")
	if err != nil {
		return "", err
	}
	var view string
	err = json.Unmarshal([]byte(jsonStr), &view)
	if err != nil {
		return "", err
	}
	return view, nil
}
