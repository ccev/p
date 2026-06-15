package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var (
	startName            string
	startDescription     string
	startWorkingDir      string
	startUser            string
	startGroup           string
	startEnv             []string
	startEnvFile         string
	startRestart         string
	startRestartSec      int
	startMemoryMax       string
	startCPUQuota        string
	startKillSignal      string
	startTimeoutStopSec  int
	startStartLimitBurst int
	startUMask           string
	startAfter           []string
	startWants           []string
	startRequires        []string
	startAutoStart       bool
	startIPAccounting    bool
	startNoStart         bool
	startReplace         bool
	startInheritPath     bool
	startInheritEnv      []string
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

var startCmd = &cobra.Command{
	Use:   "start [command...]",
	Short: "Create and start a service from a shell command",
	Long: `Create a systemd unit for the given command and start it.

Examples:
  p start "node app.js" -n web
  p start ./worker --serve -n worker -d /srv/worker
  p start "python -u bot.py" -n bot --env TOKEN=xyz --memory-max 256M`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStart,
}

func init() {
	f := startCmd.Flags()
	f.StringVarP(&startName, "name", "n", "", "service name (required)")
	f.StringVarP(&startDescription, "description", "D", "", "human-readable description")
	f.StringVarP(&startWorkingDir, "cwd", "d", "", "working directory (default: current dir)")
	f.StringVarP(&startUser, "user", "u", "", "run as user (system mode only)")
	f.StringVarP(&startGroup, "group", "g", "", "run as group (system mode only)")
	f.StringArrayVarP(&startEnv, "env", "e", nil, "environment variable KEY=VALUE (repeatable)")
	f.StringVar(&startEnvFile, "env-file", "", "load environment from file")
	f.StringVarP(&startRestart, "restart", "r", "always", "restart policy: no|on-failure|always|on-abnormal|on-watchdog|on-abort")
	f.IntVar(&startRestartSec, "restart-sec", 5, "seconds to wait before restart")
	f.StringVarP(&startMemoryMax, "memory-max", "m", "", "MemoryMax (e.g. 256M, 1G)")
	f.StringVarP(&startCPUQuota, "cpu-quota", "c", "", "CPUQuota (e.g. 50%)")
	f.StringVar(&startKillSignal, "kill-signal", "", "signal used to stop (default SIGTERM)")
	f.IntVar(&startTimeoutStopSec, "timeout-stop", 0, "TimeoutStopSec in seconds")
	f.IntVar(&startStartLimitBurst, "start-limit-burst", 0, "max restarts in burst window")
	f.StringVar(&startUMask, "umask", "", "process UMask, e.g. 0022")
	f.StringSliceVar(&startAfter, "after", nil, "After= units (comma-separated, repeatable)")
	f.StringSliceVar(&startWants, "wants", nil, "Wants= units")
	f.StringSliceVar(&startRequires, "requires", nil, "Requires= units")
	f.BoolVar(&startAutoStart, "auto-start", true, "enable unit so it starts at boot")
	f.BoolVar(&startIPAccounting, "ip-accounting", true, "enable IPAccounting so status can show network IO")
	f.BoolVar(&startNoStart, "no-start", false, "create the unit but do not start it")
	f.BoolVar(&startReplace, "force", false, "overwrite an existing service with the same name")
	f.BoolVar(&startInheritPath, "inherit-path", true, "pass the current shell's PATH into the service")
	f.StringSliceVar(&startInheritEnv, "inherit-env", nil, "additional env vars to pass from the current shell (repeatable, comma-separated)")
	_ = startCmd.MarkFlagRequired("name")
}

func runStart(cmd *cobra.Command, args []string) error {
	if !nameRE.MatchString(startName) {
		return fmt.Errorf("invalid name %q: use letters, digits, '-', '_', '.'", startName)
	}
	if systemd.Exists(startName) && !startReplace {
		return fmt.Errorf("service %q already exists; pass --force to replace, or use 'p restart %s'", startName, startName)
	}

	command := strings.Join(args, " ")
	cwd := startWorkingDir
	if cwd == "" {
		if d, err := os.Getwd(); err == nil {
			cwd = d
		}
	} else {
		abs, err := filepath.Abs(cwd)
		if err == nil {
			cwd = abs
		}
	}

	env := buildEnv(startInheritPath, startInheritEnv, startEnv)

	cfg := systemd.UnitConfig{
		Name:            startName,
		Description:     startDescription,
		Command:         command,
		WorkingDir:      cwd,
		User:            startUser,
		Group:           startGroup,
		Env:             env,
		EnvFile:         startEnvFile,
		Restart:         startRestart,
		RestartSec:      startRestartSec,
		MemoryMax:       startMemoryMax,
		CPUQuota:        startCPUQuota,
		KillSignal:      startKillSignal,
		TimeoutStopSec:  startTimeoutStopSec,
		StartLimitBurst: startStartLimitBurst,
		UMask:           startUMask,
		After:           startAfter,
		Wants:           startWants,
		Requires:        startRequires,
		IPAccounting:    startIPAccounting,
	}

	path, err := systemd.WriteUnit(cfg)
	if err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	if err := systemd.DaemonReload(); err != nil {
		return err
	}
	if startAutoStart {
		if err := systemd.Enable(startName); err != nil {
			return err
		}
	}
	if !startNoStart {
		if err := systemd.Start(startName); err != nil {
			return err
		}
	}

	fmt.Printf("%s %s\n", ui.Green.Sprint("●"), ui.Bold.Sprint(startName))
	fmt.Printf("  %s  %s\n", ui.Label.Sprint("unit "), path)
	fmt.Printf("  %s  %s\n", ui.Label.Sprint("cmd  "), command)
	if cwd != "" {
		fmt.Printf("  %s  %s\n", ui.Label.Sprint("cwd  "), cwd)
	}
	if startNoStart {
		fmt.Printf("  %s  %s\n", ui.Label.Sprint("state"), ui.Yellow.Sprint("created (not started, --no-start)"))
	} else {
		fmt.Printf("  %s  %s\n", ui.Label.Sprint("state"), ui.Green.Sprint("started"))
	}
	return nil
}

// buildEnv composes the unit's Environment= entries captured from the
// caller's shell. Static defaults (FORCE_COLOR=1 etc.) live in unit.Render()
// so they apply on `p edit` re-renders too, not just `p start`.
//
//  1. inherited vars from the caller's shell (PATH, anything in --inherit-env)
//  2. user-supplied -e entries (which win on key collision)
//
// systemd hands services a minimal PATH otherwise, which is why commands
// like `uv` or `node` (often in ~/.local/bin) are not found.
func buildEnv(inheritPath bool, inheritKeys, userEnv []string) []string {
	userKeys := map[string]bool{}
	for _, e := range userEnv {
		if i := strings.Index(e, "="); i > 0 {
			userKeys[e[:i]] = true
		}
	}
	var out []string
	add := func(key string) {
		if userKeys[key] {
			return
		}
		v, ok := os.LookupEnv(key)
		if !ok {
			return
		}
		out = append(out, key+"="+v)
	}
	if inheritPath {
		add("PATH")
	}
	for _, k := range inheritKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		add(k)
	}
	out = append(out, userEnv...)
	return out
}
