package main

import (
	"context"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

type Controller struct {
	client mqtt.Client
}

func (c Controller) run(ctx context.Context, topicToHandlers map[string][]Handler) error {
	if err := wait(ctx, c.client.Connect()); err != nil {
		return err
	}

	defer c.client.Disconnect(250)

	var topics []string
	filters := make(map[string]byte)
	for topic := range topicToHandlers {
		topics = append(topics, topic)
		filters[topic] = 0
	}

	log.WithField("filters", filters).
		Info("subscribing to topics")

	var wg sync.WaitGroup
	callback := func(client mqtt.Client, msg mqtt.Message) {
		if ctx.Err() != nil {
			return
		}

		callbackTopic := msg.Topic()
		callbackInput := string(msg.Payload())
		log.WithField("topic", callbackTopic).
			WithField("input", callbackInput).
			Info("received message from topic")

		handlers := topicToHandlers[callbackTopic]
		for _, handler := range handlers {
			wg.Add(1)
			go func(topic, input string) {
				defer wg.Done()

				log.WithField("topic", topic).
					WithField("input", input).
					Debug("handling message")
				outputs, err := handler.Handle(ctx, topic, input)
				if err != nil {
					log.WithError(err).
						WithField("topic", topic).
						WithField("input", input).
						Error("failed to handle message")
					return
				}

				for topic, output := range outputs {
					log.WithField("topic", topic).
						WithField("output", output).
						Info("publishing message to topic")
					if err := wait(ctx, client.Publish(topic, 0, false, output)); err != nil {
						log.WithError(err).
							WithField("topic", topic).
							WithField("output", output).
							Error("failed to publish message to topic")
						continue
					}
				}
			}(callbackTopic, callbackInput)
		}
	}
	if err := wait(ctx, c.client.SubscribeMultiple(filters, callback)); err != nil {
		return err
	}

	<-ctx.Done()

	if err := wait(ctx, c.client.Unsubscribe(topics...)); err != nil {
		log.WithError(err).
			WithField("topics", topics).
			Error("failed to unsubscribe from topics")
	}

	wg.Wait()

	return nil
}

func wait(ctx context.Context, token mqtt.Token) error {
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			return err
		}
	case <-ctx.Done():
		if err := token.Error(); err != nil {
			return err
		}
		return ctx.Err()
	}
	return nil
}
