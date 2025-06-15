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
		handler := AppHandler{
			key:    key,
			models: &models,
			appLib: a.appLib,
		}
		for _, topic := range app.Subscriptions {
			res.TopicToHandlers[topic] = append(res.TopicToHandlers[topic], handler)
		}
		res.KeyToHandler[key] = handler
	}

	return res, nil
}

type AppHandler struct {
	key    string
	models *sync.Map
	appLib AppLib
}

func (a AppHandler) HandleUpdate(ctx context.Context, topic string, payload string) (map[string]string, error) {
	model, ok := a.models.Load(a.key)
	if !ok {
		return nil, fmt.Errorf("model not found for app %s", a.key)
	}

	updates, err := a.appLib.update(a.key, topic, payload, model)
	if err != nil {
		return nil, err
	}

	a.models.Store(a.key, updates["model"])

	outputs := make(map[string]string)
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

func (a AppHandler) HandleView(ctx context.Context) (string, error) {
	model, ok := a.models.Load(a.key)
	if !ok {
		return "", fmt.Errorf("model not found for app %s", a.key)
	}

	view, err := a.appLib.view(a.key, model)
	if err != nil {
		return "", err
	}
	return view, nil
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

func (a AppLib) update(key string, topic string, payload string, model any) (map[string]any, error) {
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

func (a AppLib) view(key string, model any) (string, error) {
	vm := a.vm()
	vm.TLACode("config", fmt.Sprintf("import '%s'", a.config))
	vm.TLAVar("key", key)
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
