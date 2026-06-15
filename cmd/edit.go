package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	editFlagsBag  unitFlags
	editCommand   string
	editNoRestart bool
	editNoReload  bool
	editDryRun    bool
)

var declarativeFlags = []string{
	"description", "cwd", "user", "group", "env", "env-file",
	"restart", "restart-sec", "memory-max", "cpu-quota",
	"kill-signal", "timeout-stop", "start-limit-burst", "umask",
	"after", "wants", "requires", "ip-accounting",
	"unset-env", "clear-env", "cmd", "log-max",
}

var editCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Modify a service either interactively or by passing unit flags",
	Long: `With no flags, opens the service's unit file in $EDITOR (or nano / vi).
On save: daemon-reload, then restart unless --no-restart is set.

With flags (any of --cmd, --env, --restart, --memory-max, …), applies only
the explicit changes to the existing unit, then reload + restart.`,
	Args: cobra.ExactArgs(1),
	RunE: runEdit,
}

func init() {
	editCmd.Flags().StringVar(&editCommand, "cmd", "", "replace the service command (ExecStart)")
	bindEditUnitFlags(editCmd.Flags(), &editFlagsBag)
	editCmd.Flags().BoolVar(&editNoRestart, "no-restart", false, "apply changes but do not restart the service")
	editCmd.Flags().BoolVar(&editNoReload, "no-reload", false, "skip daemon-reload (testing only)")
	editCmd.Flags().BoolVar(&editDryRun, "dry-run", false, "print the new unit, change nothing on disk")
}

func runEdit(cmd *cobra.Command, args []string) error {
	name := args[0]
	if !systemd.Exists(name) {
		return fmt.Errorf("service %q not found", name)
	}
	path, err := systemd.UnitPath(name)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fs := cmd.Flags()
	var newContent string
	if declarativeMode(fs) {
		newContent, err = declarativeEdit(name, fs)
	} else {
		newContent, err = interactiveEdit(path, original)
	}
	if err != nil {
		return err
	}
	unitChanged := newContent != "" && !bytes.Equal([]byte(newContent), original)
	logMaxChanged := fs.Changed("log-max")

	ns := systemd.Namespace(name)
	confValue := ""
	confWrite := false
	switch {
	case logMaxChanged:
		confWrite = true
		confValue = editFlagsBag.logMax
	case !systemd.JournaldConfExists(ns):
		// Migration: every managed unit's Render() emits LogNamespace= now,
		// so any service without a per-namespace journald conf is missing
		// the size cap that should come with it. Write the default whenever
		// we touch this unit — that way old services pick the cap up the
		// first time the user runs `p edit` on them, even with no flags.
		confWrite = true
		confValue = systemd.DefaultLogMaxSize
	}

	if !unitChanged && !confWrite {
		fmt.Println(ui.Dim.Sprint("no changes"))
		return nil
	}
	if editDryRun {
		if unitChanged {
			fmt.Print(newContent)
		}
		if confWrite {
			fmt.Printf("# journald.conf SystemMaxUse=%s\n", confValue)
		}
		return nil
	}

	if unitChanged {
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			return err
		}
	}
	if confWrite {
		if err := systemd.WriteJournaldConf(ns, confValue); err != nil {
			return fmt.Errorf("write journald conf: %w", err)
		}
		// Best-effort: journald@<ns> may not be running yet (first activation
		// happens when the service writes its first log line).
		_ = systemd.RestartJournald(ns)
	}
	if !editNoReload {
		if err := systemd.DaemonReload(); err != nil {
			return err
		}
	}
	action := "edited"
	if unitChanged && !editNoRestart {
		if err := systemd.Restart(name); err != nil {
			return fmt.Errorf("write succeeded but restart failed: %w", err)
		}
		action = "edited & restarted"
	} else if !unitChanged && confWrite {
		if logMaxChanged {
			action = "log cap updated"
		} else {
			action = "log cap initialised"
		}
	}
	fmt.Printf("%s %s %s\n", ui.Green.Sprint("●"), ui.Bold.Sprint(name), ui.Dim.Sprint(action))
	return nil
}

func declarativeMode(fs *pflag.FlagSet) bool {
	for _, k := range declarativeFlags {
		if fs.Changed(k) {
			return true
		}
	}
	return false
}

func declarativeEdit(name string, fs *pflag.FlagSet) (string, error) {
	cfg, err := systemd.ParseUnit(name)
	if err != nil {
		return "", fmt.Errorf("parse unit: %w", err)
	}
	cfg.Name = name
	editFlagsBag.apply(cfg, fs)
	if fs.Changed("cmd") {
		cfg.Command = editCommand
	}
	if cfg.Command == "" {
		return "", fmt.Errorf("no ExecStart found and --cmd not provided")
	}
	return cfg.Render(), nil
}

func interactiveEdit(path string, original []byte) (string, error) {
	tmp, err := os.CreateTemp("", "p-edit-*.service")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(original); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		for _, candidate := range []string{"nano", "vi"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return "", fmt.Errorf("no $EDITOR set and neither nano nor vi found in PATH")
	}

	ec := exec.Command(editor, tmpPath)
	ec.Stdin = os.Stdin
	ec.Stdout = os.Stdout
	ec.Stderr = os.Stderr
	if err := ec.Run(); err != nil {
		return "", fmt.Errorf("editor exited: %w", err)
	}
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	if bytes.Equal(edited, original) {
		return "", nil
	}
	return string(edited), nil
}
