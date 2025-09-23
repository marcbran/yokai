package run

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Topic = string
type Payload = string
type Key = string

type TopicPayload struct {
	Topic   Topic
	Payload Payload
}

type Registration interface {
	Register() (Registry, error)
}

type Registry struct {
	TopicToModels map[Topic][]Model
	KeyToModel    map[Key]Model

	TopicToCommands map[Topic][]Command
}

func NewRegistry() Registry {
	return Registry{
		TopicToModels: make(map[Topic][]Model),
		KeyToModel:    make(map[Key]Model),

		TopicToCommands: make(map[Topic][]Command),
	}
}

type Plugin interface {
	Start(ctx context.Context, g *errgroup.Group, registry Registry, source Broker, sink Broker)
}

type Model interface {
	Update(ctx context.Context, topic Topic, payload Payload) (map[Topic]Payload, error)
	View(ctx context.Context) (string, error)
	SubscribeView() (<-chan string, func())
}

type Command interface {
	Command(ctx context.Context, topic Topic, payload Payload) (map[Topic]Payload, error)
}

type CompoundRegistration struct {
	registrations []Registration
}

func NewCompoundRegistration(registrations []Registration) *CompoundRegistration {
	return &CompoundRegistration{
		registrations: registrations,
	}
}

func (c CompoundRegistration) Register() (Registry, error) {
	res := NewRegistry()
	for _, registration := range c.registrations {
		registry, err := registration.Register()
		if err != nil {
			return Registry{}, err
		}
		for topic, models := range registry.TopicToModels {
			res.TopicToModels[topic] = append(res.TopicToModels[topic], models...)
		}
		for key, model := range registry.KeyToModel {
			res.KeyToModel[key] = model
		}
		for topic, commands := range registry.TopicToCommands {
			res.TopicToCommands[topic] = append(res.TopicToCommands[topic], commands...)
		}
	}
	return res, nil
}
