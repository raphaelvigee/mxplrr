package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var level string

func init() {
	rootCmd.PersistentFlags().StringVar(&level, "log", "info", "Log level")
}

var rootCmd = &cobra.Command{
	Use:           "mxplrr",
	SilenceErrors: true,
	SilenceUsage:  true,
	Short:         "Makefile explorer",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		l, err := log.ParseLevel(level)
		if err != nil {
			l = log.InfoLevel
		}

		log.SetLevel(l)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
