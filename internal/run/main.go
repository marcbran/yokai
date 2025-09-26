package run

import (
	"context"

	"golang.org/x/sync/errgroup"
)

func Run(ctx context.Context, registration Registration, plugins []Plugin) error {
	registry, err := registration.Register()
	if err != nil {
		return err
	}
	source := NewBroker()
	view := NewBroker()
	sink := NewBroker()

	g, gCtx := errgroup.WithContext(ctx)
	for _, plugin := range plugins {
		plugin.Start(gCtx, g, registry, source, view, sink)
	}

	return g.Wait()
}
