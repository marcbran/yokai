package run

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func startHttp(
	ctx context.Context,
	g *errgroup.Group,
	config HttpConfig,
	keyToModel map[Key]Model,
	broker Broker,
) {
	g.Go(func() error {
		httpCtx, httpCancel := context.WithCancel(ctx)
		defer httpCancel()

		err := runHttpServer(httpCtx, config, keyToModel, broker)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}

func runHttpServer(
	ctx context.Context,
	config HttpConfig,
	keyToModel map[Key]Model,
	broker Broker,
) error {
	mux := http.NewServeMux()

	for key, model := range keyToModel {
		mux.HandleFunc("/"+key, handleGet(model, key))
		mux.HandleFunc("/ws/"+key, handleWs(model, key, broker))
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		err := server.Shutdown(context.Background())
		if err != nil {
			log.WithError(err).
				Error("failed to shutdown server")
		}
	}()

	log.WithField("port", config.Port).
		Info("starting server")
	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func handleGet(model Model, key Key) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		view, err := model.View(r.Context())
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
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWs(model Model, key Key, broker Broker) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithError(err).
				WithField("key", key).
				Error("failed to upgrade connection to websocket")
			return
		}
		defer conn.Close()

		g, gCtx := errgroup.WithContext(r.Context())
		g.Go(func() error {
			views, unsubscribe := model.SubscribeView()
			defer unsubscribe()

			for {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				case view, ok := <-views:
					if !ok {
						return nil
					}
					err := conn.WriteMessage(websocket.TextMessage, []byte(view))
					if err != nil {
						log.WithError(err).
							WithField("key", key).
							Error("failed to write websocket message")
						return err
					}
				}
			}
		})
		g.Go(func() error {
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						log.WithError(err).
							WithField("key", key).
							Error("failed to read websocket message")
					}
					return err
				}

				updates, err := model.Update(gCtx, "viewEvents", string(message))
				if err != nil {
					log.WithError(err).
						WithField("key", key).
						Error("failed to handle websocket message")
					continue
				}

				for topic, payload := range updates {
					broker.Publish(topic, payload)
				}
			}
		})

		err = g.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).
				WithField("key", key).
				Error("websocket connection error")
		}
	}
}
