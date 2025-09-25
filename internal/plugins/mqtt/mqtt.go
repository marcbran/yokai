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
	Enabled     bool          `mapstructure:"enabled"`
	Broker      string        `mapstructure:"broker"`
	ClientId    string        `mapstructure:"clientId"`
	KeepAlive   time.Duration `mapstructure:"keep_alive"`
	PingTimeout time.Duration `mapstructure:"ping_timeout"`
}

type MqttPlugin struct {
	config Config
}

func NewPlugin(config Config) *MqttPlugin {
	return &MqttPlugin{
		config: config,
	}
}

func (m *MqttPlugin) Start(ctx context.Context, g *errgroup.Group, registry run.Registry, source run.Broker, sink run.Broker) {
	if !m.config.Enabled {
		log.Info("MQTT plugin is disabled")
		return
	}

	g.Go(func() error {
		mqttCtx, mqttCancel := context.WithCancel(ctx)
		defer mqttCancel()

		err := runMqttSub(mqttCtx, m.config, registry.TopicToModels, source)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		mqttCtx, mqttCancel := context.WithCancel(ctx)
		defer mqttCancel()

		err := runMqttPub(mqttCtx, m.config, sink)
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
	source run.Broker,
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
	err = wait(ctx, client.SubscribeMultiple(filters, func(client mqtt.Client, msg mqtt.Message) {
		if gCtx.Err() != nil {
			return
		}

		topic := msg.Topic()
		payload := string(msg.Payload())
		log.WithField("topic", topic).
			WithField("payload", payload).
			Info("received message from topic")

		source.Publish(topic, payload)
	}))
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

func runMqttPub(ctx context.Context, config Config, sink run.Broker) error {
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

	ch, unsubscribe := sink.SubscribeAll()
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
