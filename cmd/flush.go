package cmd

import (
	"fmt"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var flushAll bool

var flushCmd = &cobra.Command{
	Use:   "flush [name...]",
	Short: "Delete all journal entries for one or more services",
	Long: `Rotate the service's journal and vacuum every entry older than a second,
which in practice clears the log. Only works for units that have a
LogNamespace= set (the default on new units). Older units can be upgraded
to per-namespace logs by running 'p edit <name> --cmd …' once.`,
	RunE: runFlush,
}

func init() {
	flushCmd.Flags().BoolVar(&flushAll, "all", false, "flush every p-managed service when no name is given")
}

func runFlush(cmd *cobra.Command, args []string) error {
	names := args
	if len(names) == 0 {
		if !flushAll {
			return fmt.Errorf("specify at least one service name, or pass --all")
		}
		all, err := systemd.List()
		if err != nil {
			return err
		}
		names = all
	}
	if len(names) == 0 {
		return fmt.Errorf("no services to flush")
	}
	for _, n := range names {
		if !systemd.Exists(n) {
			return fmt.Errorf("service %q not found", n)
		}
		if err := systemd.FlushJournal(systemd.Namespace(n)); err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n",
			ui.Green.Sprint("●"),
			ui.Bold.Sprint(n),
			ui.Dim.Sprint("logs flushed"),
		)
	}
	return nil
}
