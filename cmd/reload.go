package cmd

import (
	"fmt"
	"strings"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var (
	reloadSignal string
	reloadAll    bool
)

var reloadCmd = &cobra.Command{
	Use:   "reload [name...]",
	Short: "Send a reload signal (default SIGHUP) to running services",
	Long: `Send a signal to the main PID of one or more services without restarting them.

Useful for processes that handle SIGHUP (or another signal) to re-read their
own config — nginx, haproxy, postgres, dnsmasq, …

To pick up changes to the systemd unit file itself, use 'p edit' or
'p restart' instead.`,
	RunE: runReload,
}

func init() {
	reloadCmd.Flags().StringVarP(&reloadSignal, "signal", "s", "SIGHUP", "signal to send (e.g. SIGHUP, SIGUSR1)")
	reloadCmd.Flags().BoolVar(&reloadAll, "all", false, "when no name is given, signal every p-managed service")
}

func runReload(cmd *cobra.Command, args []string) error {
	names := args
	if len(names) == 0 {
		if !reloadAll {
			return fmt.Errorf("specify at least one service name, or pass --all")
		}
		all, err := systemd.List()
		if err != nil {
			return err
		}
		names = all
	}
	if len(names) == 0 {
		return fmt.Errorf("no services to reload")
	}

	sig := reloadSignal
	if sig == "" {
		sig = "SIGHUP"
	}
	if !strings.HasPrefix(sig, "SIG") {
		sig = "SIG" + strings.ToUpper(sig)
	}

	for _, n := range names {
		if !systemd.Exists(n) {
			return fmt.Errorf("service %q not found", n)
		}
		s, err := systemd.GetStats(n)
		if err != nil {
			return err
		}
		if !s.IsRunning() || s.PID == 0 {
			return fmt.Errorf("service %q is not running — use 'p start %s' or 'p restart %s'", n, n, n)
		}
		if err := systemd.Kill(n, sig, "main"); err != nil {
			return err
		}
		fmt.Printf("%s %s %s\n",
			ui.Green.Sprint("●"),
			ui.Bold.Sprint(n),
			ui.Dim.Sprintf("reloaded (%s → pid %d)", sig, s.PID),
		)
	}
	return nil
}
