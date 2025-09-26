package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	httpplugin "github.com/marcbran/yokai/internal/plugins/http"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	config     httpplugin.Config
	httpClient *http.Client
}

func NewClient(config httpplugin.Config) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Post(ctx context.Context, topic, payload string) error {
	scheme := c.config.Scheme
	if scheme == "" {
		scheme = "http"
	}
	hostname := c.config.Hostname
	if hostname == "" {
		hostname = "localhost"
	}
	port := c.config.Port
	if port == 0 {
		port = 8000
	}
	url := fmt.Sprintf("%s://%s:%d/%s", scheme, hostname, port, topic)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Error("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) Get(ctx context.Context, view string) (string, error) {
	scheme := c.config.Scheme
	if scheme == "" {
		scheme = "http"
	}
	hostname := c.config.Hostname
	if hostname == "" {
		hostname = "localhost"
	}
	port := c.config.Port
	if port == 0 {
		port = 8000
	}
	url := fmt.Sprintf("%s://%s:%d/%s", scheme, hostname, port, view)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Error("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}
