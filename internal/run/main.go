package run

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Mqtt MqttConfig `mapstructure:"mqtt"`
	Http HttpConfig `mapstructure:"http"`
	App  AppConfig  `mapstructure:"app"`
}

type MqttConfig struct {
	Broker      string        `mapstructure:"broker"`
	ClientId    string        `mapstructure:"clientId"`
	KeepAlive   time.Duration `mapstructure:"keep_alive"`
	PingTimeout time.Duration `mapstructure:"ping_timeout"`
}

type HttpConfig struct {
	Port int `mapstructure:"port"`
}

type AppConfig struct {
	Config string `mapstructure:"config"`
}

func Run(ctx context.Context, config *Config) error {
	appLib := AppLib{config.App.Config}
	registration := CompoundRegistration{
		[]Registration{
			AppRegistration{appLib},
			CommandRegistration{},
		},
	}

	registry, err := registration.Register()
	if err != nil {
		return err
	}

	err = runWithRegistry(ctx, config, registry)
	if err != nil {
		return err
	}
	return nil
}

func runWithRegistry(ctx context.Context, config *Config, registry Registry) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return runMqttClient(ctx, config.Mqtt, registry.TopicToHandlers)
	})

	g.Go(func() error {
		return runHttpServer(ctx, config.Http, registry.KeyToHandler)
	})

	return g.Wait()
}

func runMqttClient(ctx context.Context, config MqttConfig, topicToHandlers map[string][]Handler) error {
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
				outputs, err := handler.HandleUpdate(ctx, topic, input)
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
					err := wait(ctx, client.Publish(topic, 0, false, output))
					if err != nil {
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

func runHttpServer(ctx context.Context, config HttpConfig, keyToHandler map[string]Handler) error {
	mux := http.NewServeMux()

	for key, handler := range keyToHandler {
		key := key
		handler := handler
		mux.HandleFunc("/"+key, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			view, err := handler.HandleView(r.Context())
			if err != nil {
				log.WithError(err).
					WithField("key", key).
					Error("failed to handle view")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			_, err = w.Write([]byte(view))
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		})
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		err := server.Shutdown(context.Background())
		if err != nil {
			log.WithError(err).Error("failed to shutdown server")
		}
	}()

	log.WithField("port", config.Port).Info("starting server")
	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
