package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/spf13/cobra"
)

var (
	showRaw  bool
	showNoCPU bool
)

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show full details about a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !systemd.Exists(name) {
			return fmt.Errorf("service %q not found", name)
		}
		if showRaw {
			s, err := systemd.ReadUnit(name)
			if err != nil {
				return err
			}
			fmt.Print(s)
			return nil
		}

		s, err := systemd.GetStats(name)
		if err != nil {
			return err
		}
		path, _ := systemd.UnitPath(name)
		unitText, _ := systemd.ReadUnit(name)
		execStart := extractKey(unitText, "ExecStart")
		workDir := extractKey(unitText, "WorkingDirectory")
		restart := extractKey(unitText, "Restart")
		restartSec := extractKey(unitText, "RestartSec")
		memMax := extractKey(unitText, "MemoryMax")
		cpuQuota := extractKey(unitText, "CPUQuota")
		user := extractKey(unitText, "User")
		grp := extractKey(unitText, "Group")
		envFile := extractKey(unitText, "EnvironmentFile")
		envVars := extractAll(unitText, "Environment")
		desc := s.Description
		if desc == "" {
			desc = extractKey(unitText, "Description")
		}

		var cpu float64
		if !showNoCPU && s.IsRunning() {
			cpu, _ = systemd.SampleCPUPercent(name, 250*time.Millisecond)
		}

		title := fmt.Sprintf("%s %s  %s",
			ui.StatusDot(s.Active, s.Sub),
			ui.Bold.Sprint(name),
			ui.StatusLabel(s.Active, s.Sub))

		fields := []ui.Field{
			{Label: "description", Value: orDash(desc)},
			{Label: "unit",        Value: path},
			{Label: "load",        Value: orDash(s.LoadState)},
			{Label: "enabled",     Value: orDash(s.UnitFileState)},
			{Label: "result",      Value: orDash(s.Result)},
			{Label: "pid",         Value: pidString(s.PID)},
		}
		if s.IsRunning() {
			fields = append(fields, ui.Field{Label: "uptime", Value: ui.Duration(s.Uptime())})
			fields = append(fields, ui.Field{Label: "started", Value: s.StartedAt.Local().Format(time.RFC1123)})
		}
		fields = append(fields,
			ui.Field{Label: "restarts", Value: strconv.FormatUint(uint64(s.Restarts), 10)},
			ui.Field{Label: "memory",   Value: ui.Bytes(s.MemoryBytes())},
			ui.Field{Label: "cpu",      Value: ui.Percent(cpu)},
		)
		if s.IPIngress > 0 || s.IPEgress > 0 {
			fields = append(fields, ui.Field{
				Label: "net io",
				Value: fmt.Sprintf("⇣ %s  ⇡ %s", ui.Bytes(s.IPIngress), ui.Bytes(s.IPEgress)),
			})
		}
		fields = append(fields,
			ui.Field{Label: "command", Value: orDash(execStart)},
			ui.Field{Label: "cwd",     Value: orDash(workDir)},
			ui.Field{Label: "restart", Value: fmt.Sprintf("%s (after %ss)", orDash(restart), orDefault(restartSec, "5"))},
		)
		if memMax != "" {
			fields = append(fields, ui.Field{Label: "memory max", Value: memMax})
		}
		if cpuQuota != "" {
			fields = append(fields, ui.Field{Label: "cpu quota", Value: cpuQuota})
		}
		if user != "" {
			who := user
			if grp != "" {
				who += ":" + grp
			}
			fields = append(fields, ui.Field{Label: "run as", Value: who})
		}
		if envFile != "" {
			fields = append(fields, ui.Field{Label: "env file", Value: envFile})
		}
		for _, e := range envVars {
			fields = append(fields, ui.Field{Label: "env", Value: e})
		}

		ui.RenderCard(os.Stdout, title, fields)
		return nil
	},
}

func init() {
	showCmd.Flags().BoolVar(&showRaw, "raw", false, "print the raw unit file")
	showCmd.Flags().BoolVar(&showNoCPU, "no-cpu", false, "skip CPU% sampling")
}

func extractKey(unit, key string) string {
	for _, line := range splitLines(unit) {
		if len(line) > len(key) && line[:len(key)] == key && line[len(key)] == '=' {
			return line[len(key)+1:]
		}
	}
	return ""
}

func extractAll(unit, key string) []string {
	var out []string
	for _, line := range splitLines(unit) {
		if len(line) > len(key) && line[:len(key)] == key && line[len(key)] == '=' {
			out = append(out, line[len(key)+1:])
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return ui.Dim.Sprint("—")
	}
	return s
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func pidString(p int) string {
	if p <= 0 {
		return ui.Dim.Sprint("—")
	}
	return strconv.Itoa(p)
}
