package cmd

import (
	"path/filepath"
	"strings"

	"github.com/marcbran/yokai/internal/serve"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serves the Yokai application",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetFormatter(&log.JSONFormatter{})
		configPath, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}
		config, err := loadConfig(configPath)
		if err != nil {
			return err
		}
		return serve.Serve(cmd.Context(), config)
	},
}

func init() {
	serveCmd.Flags().StringP("config", "c", "", "Path to config file")
}

func loadConfig(configPath string) (*serve.Config, error) {
	v := viper.New()

	v.SetDefault("mqtt.enabled", false)
	v.SetDefault("mqtt.client_id", "yokai")
	v.SetDefault("mqtt.keep_alive", "2s")
	v.SetDefault("mqtt.ping_timeout", "1s")
	v.SetDefault("http.enabled", false)
	v.SetDefault("http.port", 8000)
	v.SetDefault("app.config", "config.jsonnet")
	v.SetDefault("app.vendor", []string{})

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")
	_ = v.ReadInConfig()

	v.SetEnvPrefix("YOKAI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("mqtt.broker")
	_ = v.BindEnv("mqtt.client_id")
	_ = v.BindEnv("mqtt.keep_alive")
	_ = v.BindEnv("mqtt.ping_timeout")
	_ = v.BindEnv("http.port")
	_ = v.BindEnv("app.config")
	_ = v.BindEnv("app.vendor")

	var cfg serve.Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	if cfg.App.Config != "" && !filepath.IsAbs(cfg.App.Config) {
		cfg.App.Config = filepath.Join(configPath, cfg.App.Config)
	}
	if cfg.App.Vendor != nil {
		for i, vendorPath := range cfg.App.Vendor {
			if !filepath.IsAbs(vendorPath) {
				cfg.App.Vendor[i] = filepath.Join(configPath, vendorPath)
			}
		}
	}

	return &cfg, nil
}
