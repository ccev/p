package cmd

import (
	"fmt"
	"os"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm", "remove"},
	Short:   "Stop and remove a service entirely",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !systemd.Exists(name) {
			return fmt.Errorf("service %q not found", name)
		}
		_ = systemd.Stop(name)
		_ = systemd.Disable(name)
		_ = systemd.ResetFailed(name)
		if err := systemd.DeleteUnit(name); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := systemd.DaemonReload(); err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n", ui.Red.Sprint("✖"), ui.Bold.Sprint(name), ui.Dim.Sprint("deleted"))
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "ignore errors during stop/disable")
}
