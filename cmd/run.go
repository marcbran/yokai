package cmd

import (
	"github.com/marcbran/yokai/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the Yokai application",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.SetFormatter(&log.JSONFormatter{})
		return pkg.Run(cmd.Context())
	},
}
