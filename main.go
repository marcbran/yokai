package main

import (
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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
	appConfig := AppConfig{os.Getenv("YOKAI_APP_CONFIG")}
	registrations := []Registration{
		AppRegistration{appConfig},
		CommandRegistration{},
	}
	registration := CompoundRegistration{registrations}
	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(os.Getenv("YOKAI_BROKER")).
		SetClientID("yokai").
		SetKeepAlive(2 * time.Second).
		SetPingTimeout(1 * time.Second))
	controller := Controller{
		client: client,
	}

	topicToHandlers, err := registration.Register()
	if err != nil {
		return err
	}
	err = controller.run(topicToHandlers)
	if err != nil {
		return err
	}
	return nil
}

type Registration interface {
	Register() (map[string][]Handler, error)
}

type Handler interface {
	Handle(topic string, payload string) (map[string]string, error)
}

type CompoundRegistration struct {
	registrations []Registration
}

func (c CompoundRegistration) Register() (map[string][]Handler, error) {
	res := make(map[string][]Handler)
	for _, registration := range c.registrations {
		topicToHandlers, err := registration.Register()
		if err != nil {
			return nil, err
		}
		for topic, handlers := range topicToHandlers {
			res[topic] = append(res[topic], handlers...)
		}
	}
	return res, nil
}
