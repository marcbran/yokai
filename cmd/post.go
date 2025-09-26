package cmd

import (
	"fmt"

	"github.com/marcbran/yokai/internal/client"
	"github.com/spf13/cobra"
)

var postCmd = &cobra.Command{
	Use:   "post",
	Short: "Post a message to a topic",
	RunE: func(cmd *cobra.Command, args []string) error {
		topic, err := cmd.Flags().GetString("topic")
		if err != nil {
			return err
		}
		if topic == "" {
			return fmt.Errorf("topic is required")
		}

		payload, err := cmd.Flags().GetString("payload")
		if err != nil {
			return err
		}
		if payload == "" {
			return fmt.Errorf("payload is required")
		}

		configPath, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		config, err := loadConfig(configPath)
		if err != nil {
			return err
		}

		c := client.NewClient(config.Http)

		err = c.Post(cmd.Context(), topic, payload)
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	postCmd.Flags().StringP("topic", "t", "", "Topic to post to (required)")
	postCmd.Flags().StringP("payload", "p", "", "Payload to send (required)")
	postCmd.Flags().StringP("config", "c", "", "Path to config file")

	_ = postCmd.MarkFlagRequired("topic")
	_ = postCmd.MarkFlagRequired("payload")
}
