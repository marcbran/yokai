package run

import (
	"context"
	"errors"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func startMqttSub(
	ctx context.Context,
	g *errgroup.Group,
	config MqttConfig,
	topicToModels map[Topic][]Model,
	broker Broker,
) {
	g.Go(func() error {
		mqttCtx, mqttCancel := context.WithCancel(ctx)
		defer mqttCancel()

		err := runMqttSub(mqttCtx, config, topicToModels, broker)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runMqttSub(
	ctx context.Context,
	config MqttConfig,
	topicToModels map[Topic][]Model,
	broker Broker,
) error {
	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(config.Broker).
		SetClientID(config.ClientId).
		SetKeepAlive(config.KeepAlive).
		SetPingTimeout(config.PingTimeout))

	err := wait(ctx, client.Connect())
	if err != nil {
		return err
	}

	defer client.Disconnect(250)

	var topics []Topic
	filters := make(map[string]byte)
	for topic := range topicToModels {
		topics = append(topics, topic)
		filters[topic] = 0
	}

	log.WithField("filters", filters).
		Info("subscribing to topics")

	g, gCtx := errgroup.WithContext(ctx)
	callback := func(client mqtt.Client, msg mqtt.Message) {
		if gCtx.Err() != nil {
			return
		}

		callbackTopic := msg.Topic()
		callbackInput := string(msg.Payload())
		log.WithField("topic", callbackTopic).
			WithField("input", callbackInput).
			Info("received message from topic")

		models := topicToModels[callbackTopic]
		for _, model := range models {
			topic := msg.Topic()
			input := string(msg.Payload())
			g.Go(func() error {
				log.WithField("topic", topic).
					WithField("input", input).
					Debug("handling message")
				updates, err := model.Update(gCtx, topic, input)
				if err != nil {
					log.WithError(err).
						WithField("topic", topic).
						WithField("input", input).
						Error("failed to handle message")
					return err
				}

				for topic, payload := range updates {
					broker.Publish(topic, payload)
				}
				return nil
			})
		}
	}
	err = wait(ctx, client.SubscribeMultiple(filters, callback))
	if err != nil {
		return err
	}

	<-ctx.Done()

	err = wait(ctx, client.Unsubscribe(topics...))
	if err != nil {
		log.WithError(err).
			WithField("topics", topics).
			Error("failed to unsubscribe from topics")
	}

	return g.Wait()
}

func startMqttPub(ctx context.Context, g *errgroup.Group, config MqttConfig, broker Broker) {
	g.Go(func() error {
		mqttCtx, mqttCancel := context.WithCancel(ctx)
		defer mqttCancel()

		err := runMqttPub(mqttCtx, config, broker)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runMqttPub(ctx context.Context, config MqttConfig, broker Broker) error {
	client := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(config.Broker).
		SetClientID(config.ClientId).
		SetKeepAlive(config.KeepAlive).
		SetPingTimeout(config.PingTimeout))

	err := wait(ctx, client.Connect())
	if err != nil {
		return err
	}

	defer client.Disconnect(250)

	ch, unsubscribe := broker.SubscribeAll()
	defer unsubscribe()

	g, gCtx := errgroup.WithContext(ctx)

	for {
		select {
		case <-gCtx.Done():
			return g.Wait()
		case tp, ok := <-ch:
			if !ok {
				return g.Wait()
			}

			g.Go(func() error {
				log.WithField("topic", tp.Topic).
					WithField("payload", tp.Payload).
					Info("publishing message to topic")
				err := wait(gCtx, client.Publish(tp.Topic, 0, false, tp.Payload))
				if err != nil {
					log.WithError(err).
						WithField("topic", tp.Topic).
						Error("failed to publish message to topic")
				}
				return nil
			})
		}
	}
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
