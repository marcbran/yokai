package main

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
)

type CommandRegistration struct {
}

func (c CommandRegistration) Register() (map[string][]Handler, error) {
	return map[string][]Handler{
		"yokai/delay": {DelayHandler{}},
	}, nil
}

type DelayHandler struct {
}

type Delay struct {
	Milliseconds int
	Topic        string
	Message      any
}

func (d DelayHandler) Handle(topic string, payload string) (map[string]string, error) {
	var delay Delay
	err := json.Unmarshal([]byte(payload), &delay)
	if err != nil {
		return nil, err
	}

	log.WithField("milliseconds", delay.Milliseconds).
		Info("sleeping")
	time.Sleep(time.Duration(delay.Milliseconds) * time.Millisecond)

	message, err := json.Marshal(delay.Message)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		delay.Topic: string(message),
	}, nil
}
