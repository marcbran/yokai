package run

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"

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
	Config string   `mapstructure:"config"`
	Vendor []string `mapstructure:"vendor"`
}

func Run(ctx context.Context, config *Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	configPath := config.App.Config
	configDir := filepath.Dir(configPath)

	restartCh := make(chan struct{}, 1)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		watchCtx, watchCancel := context.WithCancel(gCtx)
		defer watchCancel()

		err := watchFiles(watchCtx, configDir, restartCh)
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

			err := runApp(runCtx, config)
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

func runApp(ctx context.Context, config *Config) error {
	appLib := AppLib{
		config: config.App.Config,
		vendor: config.App.Vendor,
	}
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
	source := NewBroker()
	sink := NewBroker()

	g, gCtx := errgroup.WithContext(ctx)
	startUpdater(gCtx, g, registry.TopicToModels, source, sink)
	startMqttSub(gCtx, g, config.Mqtt, registry.TopicToModels, source)
	startMqttPub(gCtx, g, config.Mqtt, sink)
	startHttp(gCtx, g, config.Http, registry.KeyToModel, sink)

	return g.Wait()
}
