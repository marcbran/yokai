package cmd

import (
	"fmt"
	"os"

	"github.com/marcbran/yokai/internal/client"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a view from the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		view, err := cmd.Flags().GetString("view")
		if err != nil {
			return err
		}
		if view == "" {
			return fmt.Errorf("view is required")
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

		response, err := c.Get(cmd.Context(), view)
		if err != nil {
			return err
		}

		_, err = os.Stdout.WriteString(response)
		if err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}

		return nil
	},
}

func init() {
	getCmd.Flags().StringP("view", "v", "", "View to retrieve (required)")
	getCmd.Flags().StringP("config", "c", "", "Path to config file")

	_ = getCmd.MarkFlagRequired("view")
}
