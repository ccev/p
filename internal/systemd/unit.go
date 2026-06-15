package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultEnv are emitted on every unit unless the user has set the same key
// via -e. They convince common toolchains (node/yarn/cargo/python rich/…)
// to keep emitting ANSI even though stdout is a journald pipe.
//
// FORCE_COLOR is set to 3 (truecolor) on purpose: many apps reach for
// `chalk.hex(...)` which silently produces no output at chalk's level 1
// (16 colors). Level 3 is what the libraries themselves want when they
// detect a TTY, and any caller that only treats FORCE_COLOR as truthy
// also gets the right behaviour.
//
// The parser strips entries with these keys from cfg.Env on read so that
// each Render() emits the *current* default value — otherwise old units
// would silently keep stale defaults forever (e.g. FORCE_COLOR=1 sticking
// around after we bumped the default to 3).
var defaultEnv = []string{
	"FORCE_COLOR=3",
	"CLICOLOR_FORCE=1",
	"PY_COLORS=1",
}

// IsManagedDefaultEnv reports whether an Environment= entry's KEY is one
// of p's managed defaults. Used by the parser to strip those entries on
// load so re-renders pick up the latest default values.
func IsManagedDefaultEnv(entry string) bool {
	key := entry
	if i := strings.Index(entry, "="); i > 0 {
		key = entry[:i]
	}
	for _, d := range defaultEnv {
		eq := strings.Index(d, "=")
		if eq > 0 && d[:eq] == key {
			return true
		}
	}
	return false
}

type UnitConfig struct {
	Name            string
	Description     string
	Command         string
	WorkingDir      string
	User            string
	Group           string
	Env             []string
	EnvFile         string
	Restart         string
	RestartSec      int
	MemoryMax       string
	CPUQuota        string
	StandardOutput  string
	StandardError   string
	KillSignal      string
	TimeoutStopSec  int
	After           []string
	Wants           []string
	Requires        []string
	IPAccounting    bool
	StartLimitBurst int
	UMask           string
	// Shell is the absolute path to the login shell used to wrap Command.
	// When empty, Render() falls back to $SHELL, then /bin/bash.
	Shell string
}

func (c UnitConfig) Render() string {
	var b strings.Builder
	b.WriteString("# Managed by p — https://github.com/ccev/p\n")
	b.WriteString("[Unit]\n")
	desc := c.Description
	if desc == "" {
		desc = "p-managed service: " + c.Name
	}
	fmt.Fprintf(&b, "Description=%s\n", desc)
	if len(c.After) > 0 {
		fmt.Fprintf(&b, "After=%s\n", strings.Join(c.After, " "))
	}
	if len(c.Wants) > 0 {
		fmt.Fprintf(&b, "Wants=%s\n", strings.Join(c.Wants, " "))
	}
	if len(c.Requires) > 0 {
		fmt.Fprintf(&b, "Requires=%s\n", strings.Join(c.Requires, " "))
	}
	if c.StartLimitBurst > 0 {
		fmt.Fprintf(&b, "StartLimitBurst=%d\n", c.StartLimitBurst)
	}

	b.WriteString("\n[Service]\n")
	b.WriteString("Type=simple\n")
	shell := resolveShell(c.Shell)
	wrapped := "exec " + strings.TrimPrefix(c.Command, "exec ")
	fmt.Fprintf(&b, "ExecStart=%s -lc %s\n", shell, shellQuote(wrapped))
	if c.WorkingDir != "" {
		fmt.Fprintf(&b, "WorkingDirectory=%s\n", c.WorkingDir)
	}
	if c.User != "" {
		fmt.Fprintf(&b, "User=%s\n", c.User)
	}
	if c.Group != "" {
		fmt.Fprintf(&b, "Group=%s\n", c.Group)
	}
	existingEnv := map[string]bool{}
	for _, e := range c.Env {
		if i := strings.Index(e, "="); i > 0 {
			existingEnv[e[:i]] = true
		}
	}
	for _, kv := range defaultEnv {
		eq := strings.Index(kv, "=")
		if eq <= 0 || existingEnv[kv[:eq]] {
			continue
		}
		fmt.Fprintf(&b, "Environment=%s\n", kv)
	}
	for _, e := range c.Env {
		fmt.Fprintf(&b, "Environment=%s\n", e)
	}
	if c.EnvFile != "" {
		fmt.Fprintf(&b, "EnvironmentFile=%s\n", c.EnvFile)
	}
	restart := c.Restart
	if restart == "" {
		restart = "always"
	}
	fmt.Fprintf(&b, "Restart=%s\n", restart)
	rs := c.RestartSec
	if rs <= 0 {
		rs = 5
	}
	fmt.Fprintf(&b, "RestartSec=%d\n", rs)
	if c.MemoryMax != "" {
		fmt.Fprintf(&b, "MemoryMax=%s\n", c.MemoryMax)
	}
	if c.CPUQuota != "" {
		fmt.Fprintf(&b, "CPUQuota=%s\n", c.CPUQuota)
	}
	if c.StandardOutput != "" {
		fmt.Fprintf(&b, "StandardOutput=%s\n", c.StandardOutput)
	}
	if c.StandardError != "" {
		fmt.Fprintf(&b, "StandardError=%s\n", c.StandardError)
	}
	if c.KillSignal != "" {
		fmt.Fprintf(&b, "KillSignal=%s\n", c.KillSignal)
	}
	if c.TimeoutStopSec > 0 {
		fmt.Fprintf(&b, "TimeoutStopSec=%d\n", c.TimeoutStopSec)
	}
	if c.UMask != "" {
		fmt.Fprintf(&b, "UMask=%s\n", c.UMask)
	}
	if c.IPAccounting {
		b.WriteString("IPAccounting=yes\n")
	}
	fmt.Fprintf(&b, "LogNamespace=%s\n", Namespace(c.Name))

	b.WriteString("\n[Install]\n")
	if CurrentMode() == ModeSystem {
		b.WriteString("WantedBy=multi-user.target\n")
	} else {
		b.WriteString("WantedBy=default.target\n")
	}
	return b.String()
}

func WriteUnit(c UnitConfig) (string, error) {
	dir, err := CurrentMode().UnitDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, ServiceUnit(c.Name))
	if err := os.WriteFile(path, []byte(c.Render()), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func DeleteUnit(name string) error {
	dir, err := CurrentMode().UnitDir()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, ServiceUnit(name)))
}

func UnitPath(name string) (string, error) {
	dir, err := CurrentMode().UnitDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ServiceUnit(name)), nil
}

func ReadUnit(name string) (string, error) {
	path, err := UnitPath(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// resolveShell picks the shell whose rc files we want sourced for the service.
// Preferring the caller's $SHELL means services inherit the same setup
// (PATH tweaks, nvm shims, direnv hooks, locale, …) as an interactive
// terminal — without having to bake every variable into the unit.
func resolveShell(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}
