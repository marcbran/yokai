package main

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

type Controller struct {
	client mqtt.Client
}

func (c Controller) run(topicToHandlers map[string][]Handler) error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	defer c.client.Disconnect(250)

	filters := make(map[string]byte)
	for topic := range topicToHandlers {
		filters[topic] = 0
	}

	log.WithField("filters", filters).
		Info("subscribing to topics")

	c.client.SubscribeMultiple(filters, func(client mqtt.Client, msg mqtt.Message) {
		topic := msg.Topic()
		input := string(msg.Payload())
		log.WithField("topic", topic).
			WithField("input", input).
			Info("received message from topic")

		handlers := topicToHandlers[topic]
		for _, handler := range handlers {
			go func(topic, input string) {
				outputs, err := handler.Handle(topic, input)
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
					if token := c.client.Publish(topic, 0, false, output); token.Wait() && token.Error() != nil {
						log.WithError(err).
							WithField("topic", topic).
							WithField("output", output).
							Error("failed to publish message to topic")
						continue
					}
				}
			}(topic, input)
		}
	})

	done := make(chan struct{})
	<-done
	return nil
}
