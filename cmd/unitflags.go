package cmd

import (
	"strings"

	"github.com/ccev/p/internal/systemd"
	"github.com/spf13/pflag"
)

// unitFlags holds the subset of flags that describe a systemd unit's
// definition (not its lifecycle). Used by `edit` to apply additive changes
// to an existing UnitConfig parsed from disk.
type unitFlags struct {
	description     string
	cwd             string
	user            string
	group           string
	env             []string
	unsetEnv        []string
	clearEnv        bool
	envFile         string
	restart         string
	restartSec      int
	memoryMax       string
	cpuQuota        string
	killSignal      string
	timeoutStopSec  int
	startLimitBurst int
	umask           string
	after           []string
	wants           []string
	requires        []string
	ipAccounting    bool
	logMax          string
}

// bindEditUnitFlags registers the unit-definition flags on fs with zero
// defaults so we can use FlagSet.Changed() to detect explicit overrides.
func bindEditUnitFlags(fs *pflag.FlagSet, f *unitFlags) {
	fs.StringVarP(&f.description, "description", "D", "", "set Description=")
	fs.StringVarP(&f.cwd, "cwd", "d", "", "set WorkingDirectory=")
	fs.StringVarP(&f.user, "user", "u", "", "set User=")
	fs.StringVarP(&f.group, "group", "g", "", "set Group=")
	fs.StringArrayVarP(&f.env, "env", "e", nil, "append Environment= entries (repeatable)")
	fs.StringSliceVar(&f.unsetEnv, "unset-env", nil, "remove environment keys")
	fs.BoolVar(&f.clearEnv, "clear-env", false, "clear all Environment= entries before applying --env")
	fs.StringVar(&f.envFile, "env-file", "", "set EnvironmentFile=")
	fs.StringVarP(&f.restart, "restart", "r", "", "set Restart=")
	fs.IntVar(&f.restartSec, "restart-sec", 0, "set RestartSec=")
	fs.StringVarP(&f.memoryMax, "memory-max", "m", "", "set MemoryMax=")
	fs.StringVarP(&f.cpuQuota, "cpu-quota", "c", "", "set CPUQuota=")
	fs.StringVar(&f.killSignal, "kill-signal", "", "set KillSignal=")
	fs.IntVar(&f.timeoutStopSec, "timeout-stop", 0, "set TimeoutStopSec= (seconds before SIGKILL during stop/restart)")
	fs.IntVar(&f.startLimitBurst, "start-limit-burst", 0, "set StartLimitBurst=")
	fs.StringVar(&f.umask, "umask", "", "set UMask=")
	fs.StringSliceVar(&f.after, "after", nil, "replace After=")
	fs.StringSliceVar(&f.wants, "wants", nil, "replace Wants=")
	fs.StringSliceVar(&f.requires, "requires", nil, "replace Requires=")
	fs.BoolVar(&f.ipAccounting, "ip-accounting", false, "set IPAccounting=")
	fs.StringVar(&f.logMax, "log-max", "", "max disk usage for this service's logs (e.g. 20M, 1G)")
}

// apply mutates cfg in-place, only touching fields whose flags were
// explicitly set on the command line.
func (f *unitFlags) apply(cfg *systemd.UnitConfig, fs *pflag.FlagSet) {
	if fs.Changed("description") {
		cfg.Description = f.description
	}
	if fs.Changed("cwd") {
		cfg.WorkingDir = f.cwd
	}
	if fs.Changed("user") {
		cfg.User = f.user
	}
	if fs.Changed("group") {
		cfg.Group = f.group
	}
	if f.clearEnv {
		cfg.Env = nil
	}
	if len(f.env) > 0 {
		cfg.Env = append(cfg.Env, f.env...)
	}
	if len(f.unsetEnv) > 0 {
		cfg.Env = removeEnv(cfg.Env, f.unsetEnv)
	}
	if fs.Changed("env-file") {
		cfg.EnvFile = f.envFile
	}
	if fs.Changed("restart") {
		cfg.Restart = f.restart
	}
	if fs.Changed("restart-sec") {
		cfg.RestartSec = f.restartSec
	}
	if fs.Changed("memory-max") {
		cfg.MemoryMax = f.memoryMax
	}
	if fs.Changed("cpu-quota") {
		cfg.CPUQuota = f.cpuQuota
	}
	if fs.Changed("kill-signal") {
		cfg.KillSignal = f.killSignal
	}
	if fs.Changed("timeout-stop") {
		cfg.TimeoutStopSec = f.timeoutStopSec
	}
	if fs.Changed("start-limit-burst") {
		cfg.StartLimitBurst = f.startLimitBurst
	}
	if fs.Changed("umask") {
		cfg.UMask = f.umask
	}
	if fs.Changed("after") {
		cfg.After = f.after
	}
	if fs.Changed("wants") {
		cfg.Wants = f.wants
	}
	if fs.Changed("requires") {
		cfg.Requires = f.requires
	}
	if fs.Changed("ip-accounting") {
		cfg.IPAccounting = f.ipAccounting
	}
}

func removeEnv(env, remove []string) []string {
	if len(env) == 0 {
		return env
	}
	rm := make(map[string]bool, len(remove))
	for _, k := range remove {
		rm[k] = true
	}
	out := env[:0]
	for _, e := range env {
		key := e
		if i := strings.Index(e, "="); i > 0 {
			key = e[:i]
		}
		if !rm[key] {
			out = append(out, e)
		}
	}
	return out
}
