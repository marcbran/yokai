package inout

import (
	"context"
	"errors"

	"github.com/marcbran/yokai/internal/run"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type InoutPlugin struct {
	inputs  []run.TopicPayload
	outputs chan run.TopicPayload
}

func NewPlugin(inputs []run.TopicPayload) *InoutPlugin {
	return &InoutPlugin{
		inputs:  inputs,
		outputs: make(chan run.TopicPayload, 100),
	}
}

func (i *InoutPlugin) Outputs() <-chan run.TopicPayload {
	return i.outputs
}

func (i *InoutPlugin) Start(ctx context.Context, g *errgroup.Group, registry run.Registry, source run.Broker, sink run.Broker) {
	g.Go(func() error {
		inCtx, inCancel := context.WithCancel(ctx)
		defer inCancel()

		err := runIn(inCtx, i, source)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		outCtx, outCancel := context.WithCancel(ctx)
		defer outCancel()

		err := runOut(outCtx, i, sink)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runIn(
	ctx context.Context,
	plugin *InoutPlugin,
	source run.Broker,
) error {
	for _, tp := range plugin.inputs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			log.WithField("topic", tp.Topic).
				WithField("payload", tp.Payload).
				Info("inputting message to source")
			source.Publish(tp.Topic, tp.Payload)
		}
	}
	return nil
}

func runOut(
	ctx context.Context,
	plugin *InoutPlugin,
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
				Info("collecting output from sink")

			select {
			case plugin.outputs <- tp:
			default:
			}
		}
	}
}
