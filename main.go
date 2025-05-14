package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(os.Getenv("YOKAI_BROKER")).
		SetClientID("yokai").
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second))
	appConfig := NewAppConfig(os.Getenv("YOKAI_APP_CONFIG"))
	controller := NewController(client, appConfig)

	err := controller.run()
	if err != nil {
		return err
	}
	return nil
}

type Controller struct {
	client    mqtt.Client
	appConfig AppConfig
}

func NewController(
	client mqtt.Client,
	appConfig AppConfig,
) Controller {
	return Controller{
		client:    client,
		appConfig: appConfig,
	}
}

func (c Controller) run() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	defer c.client.Disconnect(250)
	err := c.configure()
	if err != nil {
		return err
	}
	done := make(chan struct{})
	<-done
	return nil
}

func (c Controller) configure() error {
	err := c.configureDelay()
	if err != nil {
		return err
	}
	err = c.configureApps()
	if err != nil {
		return err
	}
	return nil
}

type Delay struct {
	Milliseconds int
	Topic        string
	Message      any
}

func (c Controller) configureDelay() error {
	topic := "yokai/delay"
	if token := c.client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		go func() {
			payload := msg.Payload()
			log.WithField("topic", topic).
				WithField("payload", string(payload)).
				Info("received message from topic")
			var delay Delay
			err := json.Unmarshal(payload, &delay)
			if err != nil {
				log.Error(err)
				return
			}
			log.WithField("milliseconds", delay.Milliseconds).
				Info("sleeping")
			time.Sleep(time.Duration(delay.Milliseconds) * time.Millisecond)
			log.WithField("topic", delay.Topic).
				WithField("message", delay.Message).
				Info("publishing message")
			messageJson, err := json.Marshal(delay.Message)
			if err != nil {
				log.Error(err)
				return
			}
			if token := c.client.Publish(delay.Topic, 0, false, messageJson); token.Wait() && token.Error() != nil {
				log.Error(err)
				return
			}
		}()
	}); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	log.WithField("topic", topic).
		Info("subscribe to topic")
	return nil
}

func (c Controller) configureApps() error {
	apps, err := c.appConfig.listApps()
	if err != nil {
		return err
	}

	var models []any
	filters := make(map[string]byte)
	topicToAppIndices := make(map[string][]int)
	for i, app := range apps {
		models = append(models, app.Model)
		for _, topic := range app.Subscriptions {
			filters[topic] = 0
			topicToAppIndices[topic] = append(topicToAppIndices[topic], i)
		}
	}

	log.WithField("filters", filters).
		Info("subscribing to topics")

	c.client.SubscribeMultiple(filters, func(client mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		payload := string(msg.Payload())
		log.WithField("topic", topic).
			WithField("payload", payload).
			Info("received message from topic")

		appIndices := topicToAppIndices[msg.Topic()]
		for _, index := range appIndices {
			model := models[index]
			updates, err := c.appConfig.update(index, topic, model, payload)
			if err != nil {
				log.Error(err)
				continue
			}
			models[index] = updates["model"]

			for topic, payload := range updates {
				if topic == "model" {
					continue
				}
				log.WithField("topic", topic).
					WithField("payload", payload).
					Info("publishing message to topic")

				b, err := json.Marshal(payload)
				if err != nil {
					log.Error(err)
					continue
				}
				if token := c.client.Publish(topic, 0, false, string(b)); token.Wait() && token.Error() != nil {
					log.Error(err)
					continue
				}
			}
		}
	})
	return nil
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
