package cmd

import (
	"strings"

	"github.com/marcbran/yokai/internal/run"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the Yokai application",
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
		return run.Run(cmd.Context(), config)
	},
}

func init() {
	runCmd.Flags().StringP("config", "c", "", "Path to config file")
}

func loadConfig(configPath string) (*run.Config, error) {
	v := viper.New()

	v.SetDefault("mqtt.client_id", "yokai")
	v.SetDefault("mqtt.keep_alive", "2s")
	v.SetDefault("mqtt.ping_timeout", "1s")
	v.SetDefault("http.port", 8000)

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

	var cfg run.Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
