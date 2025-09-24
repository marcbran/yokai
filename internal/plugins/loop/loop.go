package loop

import (
	"context"
	"errors"

	"github.com/marcbran/yokai/internal/run"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type LoopPlugin struct{}

func NewPlugin() *LoopPlugin {
	return &LoopPlugin{}
}

func (l *LoopPlugin) Start(ctx context.Context, g *errgroup.Group, registry run.Registry, source run.Broker, sink run.Broker) {
	g.Go(func() error {
		loopCtx, loopCancel := context.WithCancel(ctx)
		defer loopCancel()

		err := runLoop(loopCtx, source, sink)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runLoop(
	ctx context.Context,
	source run.Broker,
	sink run.Broker,
) error {
	ch, unsubscribe := sink.SubscribeAll()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case tp, ok := <-ch:
			if !ok {
				return nil
			}

			log.WithField("topic", tp.Topic).
				WithField("payload", tp.Payload).
				Info("looping message from sink to source")

			source.Publish(tp.Topic, tp.Payload)
		}
	}
}
