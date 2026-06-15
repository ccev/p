package cmd

import (
	"fmt"
	"os"

	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "p",
	Short:         "p — friendly systemd wrapper inspired by pm2",
	Long:          "p is a thin wrapper around systemd that makes running long-lived processes feel like pm2.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Red.Sprint("error: ")+err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		startCmd,
		stopCmd,
		restartCmd,
		reloadCmd,
		deleteCmd,
		statusCmd,
		logsCmd,
		showCmd,
		editCmd,
		flushCmd,
	)
}
