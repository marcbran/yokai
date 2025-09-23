package serve

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/marcbran/yokai/internal/plugins/http"
	"github.com/marcbran/yokai/internal/plugins/mqtt"
	"github.com/marcbran/yokai/internal/run"
	log "github.com/sirupsen/logrus"

	"golang.org/x/sync/errgroup"
)

type Config struct {
	Mqtt mqtt.Config   `mapstructure:"mqtt"`
	Http http.Config   `mapstructure:"http"`
	App  run.AppConfig `mapstructure:"app"`
}

func Serve(ctx context.Context, config *Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	registration := run.NewCompoundRegistration(
		[]run.Registration{
			run.NewAppRegistration(config.App),
			run.CommandRegistration{},
		},
	)
	plugins := []run.Plugin{
		run.NewUpdaterPlugin(),
		mqtt.NewMqttPlugin(config.Mqtt),
		http.NewHttpPlugin(config.Http),
	}

	configPath := config.App.Config
	configDir := filepath.Dir(configPath)
	return reloadOnFileChanges(ctx, configDir, func(ctx context.Context) error {
		return run.Run(ctx, registration, plugins)
	})
}

func reloadOnFileChanges(ctx context.Context, dir string, body func(ctx context.Context) error) error {
	restartCh := make(chan struct{}, 1)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		watchCtx, watchCancel := context.WithCancel(gCtx)
		defer watchCancel()

		err := watchFiles(watchCtx, dir, restartCh)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		for {
			if gCtx.Err() != nil {
				return gCtx.Err()
			}

			runCtx, runCancel := context.WithCancel(gCtx)

			go func() {
				select {
				case <-restartCh:
					runCancel()
				case <-runCtx.Done():
				}
			}()

			err := body(runCtx)
			runCancel()

			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	})

	return g.Wait()
}

func watchFiles(ctx context.Context, dir string, restartCh chan<- struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() {
		err := watcher.Close()
		if err != nil {
			log.WithError(err).
				Error("failed to close watcher")
		}
	}()

	err = watcher.Add(dir)
	if err != nil {
		return err
	}

	log.WithField("directory", dir).
		Info("watching directory for changes")

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				ext := filepath.Ext(event.Name)
				if ext == ".jsonnet" || ext == ".libsonnet" {
					log.WithField("file", event.Name).
						Info("file changed, triggering restart")
					select {
					case restartCh <- struct{}{}:
					default:
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.WithError(err).
				Error("file watcher error")
		}
	}
}
