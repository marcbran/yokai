package mqtt

import (
	"context"
	"errors"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/marcbran/yokai/internal/run"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Broker      string        `mapstructure:"broker"`
	ClientId    string        `mapstructure:"clientId"`
	KeepAlive   time.Duration `mapstructure:"keep_alive"`
	PingTimeout time.Duration `mapstructure:"ping_timeout"`
}

type MqttPlugin struct {
	config Config
}

func NewMqttPlugin(config Config) *MqttPlugin {
	return &MqttPlugin{
		config: config,
	}
}

func (m *MqttPlugin) Start(ctx context.Context, g *errgroup.Group, registry run.Registry, source run.Broker, sink run.Broker) {
	g.Go(func() error {
		mqttCtx, mqttCancel := context.WithCancel(ctx)
		defer mqttCancel()

		err := runMqttSub(mqttCtx, m.config, registry.TopicToModels, sink)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		mqttCtx, mqttCancel := context.WithCancel(ctx)
		defer mqttCancel()

		err := runMqttPub(mqttCtx, m.config, source)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runMqttSub(
	ctx context.Context,
	config Config,
	topicToModels map[run.Topic][]run.Model,
	broker run.Broker,
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

	var topics []run.Topic
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

func runMqttPub(ctx context.Context, config Config, broker run.Broker) error {
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
