package run

import "context"

type Registration interface {
	Register() (Registry, error)
}

type Registry struct {
	TopicToHandlers map[string][]Handler
	KeyToHandler    map[string]Handler
}

func NewRegistry() Registry {
	return Registry{
		TopicToHandlers: make(map[string][]Handler),
		KeyToHandler:    make(map[string]Handler),
	}
}

type Handler interface {
	HandleUpdate(ctx context.Context, topic string, payload string) (map[string]string, error)
	HandleView(ctx context.Context) (string, error)
}

type CompoundRegistration struct {
	registrations []Registration
}

func (c CompoundRegistration) Register() (Registry, error) {
	res := NewRegistry()
	for _, registration := range c.registrations {
		registry, err := registration.Register()
		if err != nil {
			return Registry{}, err
		}
		for topic, handlers := range registry.TopicToHandlers {
			res.TopicToHandlers[topic] = append(res.TopicToHandlers[topic], handlers...)
		}
		for key, handler := range registry.KeyToHandler {
			res.KeyToHandler[key] = handler
		}
	}
	return res, nil
}
