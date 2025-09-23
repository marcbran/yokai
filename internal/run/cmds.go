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
		TopicToCommands: map[Topic][]Command{
			"yokai/delay": {DelayCommand{}},
		},
	}, nil
}

type DelayCommand struct {
}

type Delay struct {
	Milliseconds int
	Topic        Topic
	Message      any
}

func (d DelayCommand) Command(ctx context.Context, topic Topic, payload Payload) (map[Topic]Payload, error) {
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
	return map[Topic]Payload{
		delay.Topic: string(message),
	}, nil
}
