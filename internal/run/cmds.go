package run

import (
	"context"
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

func (d DelayHandler) Handle(ctx context.Context, topic string, payload string) (map[string]string, error) {
	var delay Delay
	err := json.Unmarshal([]byte(payload), &delay)
	if err != nil {
		return nil, err
	}

	log.WithField("milliseconds", delay.Milliseconds).
		Info("sleeping")
	select {
	case <-time.After(time.Duration(delay.Milliseconds) * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	message, err := json.Marshal(delay.Message)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		delay.Topic: string(message),
	}, nil
}
