package run

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type UpdaterPlugin struct{}

func NewUpdaterPlugin() *UpdaterPlugin {
	return &UpdaterPlugin{}
}

func (u *UpdaterPlugin) Start(ctx context.Context, g *errgroup.Group, registry Registry, source Broker, view Broker, sink Broker) {
	g.Go(func() error {
		updaterCtx, updaterCancel := context.WithCancel(ctx)
		defer updaterCancel()

		err := runUpdater(updaterCtx, registry.TopicToModels, source, view, sink)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runUpdater(
	ctx context.Context,
	topicToModels map[Topic][]Model,
	source Broker,
	view Broker,
	sink Broker,
) error {
	g, gCtx := errgroup.WithContext(ctx)

	for topic, models := range topicToModels {
		topic := topic
		models := models

		g.Go(func() error {
			ch, unsubscribe := source.Subscribe(topic)
			defer unsubscribe()

			for {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				case payload, ok := <-ch:
					if !ok {
						return nil
					}

					log.WithField("topic", topic).
						WithField("payload", payload).
						Info("received message from topic")

					var allViews []TopicPayload
					var allCommands []TopicPayload

					for _, model := range models {
						commands, err := model.Update(gCtx, topic, payload)
						if err != nil {
							log.WithError(err).
								WithField("topic", topic).
								WithField("payload", payload).
								Error("failed to handle message")
							continue
						}
						for topic, payload := range commands {
							allCommands = append(allCommands, TopicPayload{
								Topic:   topic,
								Payload: payload,
							})
						}

						view, err := model.View(gCtx)
						if err != nil {
							log.WithError(err).
								WithField("topic", topic).
								WithField("payload", payload).
								Error("failed to render view")
							continue
						}
						allViews = append(allViews, TopicPayload{
							Topic:   model.Key(),
							Payload: view,
						})
					}

					for _, v := range allViews {
						log.WithField("key", v.Topic).
							WithField("view", v.Payload).
							Info("publishing view to key")
						view.Publish(v.Topic, v.Payload)
					}

					for _, cmd := range allCommands {
						log.WithField("topic", cmd.Topic).
							WithField("payload", cmd.Payload).
							Info("publishing command to topic")
						sink.Publish(cmd.Topic, cmd.Payload)
					}
				}
			}
		})
	}

	return g.Wait()
}
