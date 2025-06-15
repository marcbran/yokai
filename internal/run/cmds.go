package run

import (
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
)

type CommandRegistration struct {
}

func (c CommandRegistration) Register() (Registry, error) {
	return Registry{
		TopicToHandlers: map[string][]Handler{
			"yokai/delay": {DelayHandler{}},
		},
		KeyToHandler: nil,
	}, nil
}

type CommandHandler struct {
}

func (d CommandHandler) HandleUpdate(ctx context.Context, topic string, payload string) (map[string]string, error) {
	return nil, nil
}

func (d CommandHandler) HandleView(ctx context.Context) (string, error) {
	return "", nil
}

func (d CommandHandler) HandleViewEvent(ctx context.Context, payload string) (map[string]string, error) {
	return nil, nil
}

func (d CommandHandler) SubscribeView() (<-chan string, func()) {
	return nil, nil
}

type DelayHandler struct {
	CommandHandler
}

type Delay struct {
	Milliseconds int
	Topic        string
	Message      any
}

func (d DelayHandler) HandleUpdate(ctx context.Context, topic string, payload string) (map[string]string, error) {
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
