package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/go-jsonnet"
)

type AppRegistration struct {
	appConfig AppConfig
}

func (a AppRegistration) Register() (map[string][]Handler, error) {
	apps, err := a.appConfig.listApps()
	if err != nil {
		return nil, err
	}

	var models sync.Map
	topicToHandlers := make(map[string][]Handler)
	for i, app := range apps {
		models.Store(i, app.Model)
		for _, topic := range app.Subscriptions {
			topicToHandlers[topic] = append(topicToHandlers[topic], AppHandler{
				index:     i,
				models:    &models,
				appConfig: a.appConfig,
			})
		}
	}

	return topicToHandlers, nil
}

type AppHandler struct {
	index     int
	models    *sync.Map
	appConfig AppConfig
}

func (a AppHandler) Handle(ctx context.Context, topic string, payload string) (map[string]string, error) {
	model, ok := a.models.Load(a.index)
	if !ok {
		return nil, fmt.Errorf("model not found for app %d", a.index)
	}

	updates, err := a.appConfig.update(a.index, topic, model, payload)
	if err != nil {
		return nil, err
	}

	a.models.Store(a.index, updates["model"])

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

type AppConfig struct {
	path string
}

func NewAppConfig(path string) AppConfig {
	return AppConfig{
		path: path,
	}
}

type AppData struct {
	Model         string   `json:"model"`
	Init          any      `json:"init"`
	Subscriptions []string `json:"subscriptions"`
}

func (a AppConfig) listApps() ([]AppData, error) {
	var apps []AppData
	err := evaluateAndUnmarshal("apps", fmt.Sprintf(`
		local apps = import "%s";
		std.map(function (app) {
			model: std.get(app.app, "model", ""),
			init: std.get(app.app, "init", ""),
			subscriptions: std.get(app.app, "subscriptions", []),
		}, apps)
	`, a.path), &apps)
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (a AppConfig) update(index int, topic string, model any, payload any) (map[string]any, error) {
	var update map[string]any
	err := evaluateAndUnmarshal("update", fmt.Sprintf(`
		local apps = import "%s";
		local index = %d;
		local topic = "%s";
		local model = "%s";
		local payload = %s;
		apps[index].app.update[topic](model, payload)
	`, a.path, index, topic, model, payload), &update)
	if err != nil {
		return nil, err
	}
	return update, nil
}

func evaluateAndUnmarshal(name, snippet string, v any) error {
	vm := jsonnet.MakeVM()
	jsonStr, err := vm.EvaluateAnonymousSnippet(name, snippet)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(jsonStr), v)
}
