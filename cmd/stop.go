package cmd

import (
	"fmt"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a running service (keeps the unit)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !systemd.Exists(name) {
			return fmt.Errorf("service %q not found", name)
		}
		if err := systemd.Stop(name); err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n", ui.Yellow.Sprint("●"), ui.Bold.Sprint(name), ui.Dim.Sprint("stopped"))
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart <name>",
	Short: "Restart a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !systemd.Exists(name) {
			return fmt.Errorf("service %q not found", name)
		}
		if err := systemd.Restart(name); err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n", ui.Green.Sprint("●"), ui.Bold.Sprint(name), ui.Dim.Sprint("restarted"))
		return nil
	},
}
