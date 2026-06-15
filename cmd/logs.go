package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ccev/p/internal/systemd"
	"github.com/ccev/p/internal/ui"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	logsLines    int
	logsNoFollow bool
	logsNoColor  bool
	logsRaw      bool
	logsSince    string
	logsGrep     string
)

var logsCmd = &cobra.Command{
	Use:     "logs [name...]",
	Aliases: []string{"log"},
	Short:   "Stream live, colored logs from one or more services",
	RunE:    runLogs,
}

func init() {
	f := logsCmd.Flags()
	f.IntVarP(&logsLines, "lines", "l", 50, "number of initial lines to print")
	f.BoolVar(&logsNoFollow, "no-follow", false, "print existing lines and exit")
	f.BoolVar(&logsNoColor, "no-color", false, "disable colored output")
	f.BoolVar(&logsRaw, "raw", false, "do not prefix lines with service name")
	f.StringVarP(&logsSince, "since", "s", "", "show logs since (e.g. \"10 min ago\", \"2025-01-01\")")
	f.StringVar(&logsGrep, "grep", "", "only show lines matching this regex")
}

var (
	logLevelRE = regexp.MustCompile(`(?i)\b(error|err|fatal|panic|warn|warning|info|debug|trace)\b`)
	prefixPalette = []*color.Color{
		color.New(color.FgCyan),
		color.New(color.FgMagenta),
		color.New(color.FgGreen),
		color.New(color.FgYellow),
		color.New(color.FgBlue),
		color.New(color.FgRed),
		color.New(color.FgHiCyan),
		color.New(color.FgHiMagenta),
	}
)

func runLogs(cmd *cobra.Command, args []string) error {
	if logsNoColor {
		color.NoColor = true
	}
	names := args
	if len(names) == 0 {
		all, err := systemd.List()
		if err != nil {
			return err
		}
		names = all
	}
	if len(names) == 0 {
		return fmt.Errorf("no services to tail")
	}
	for _, n := range names {
		if !systemd.Exists(n) {
			return fmt.Errorf("service %q not found", n)
		}
	}

	var grepRE *regexp.Regexp
	if logsGrep != "" {
		re, err := regexp.Compile(logsGrep)
		if err != nil {
			return fmt.Errorf("invalid --grep regex: %w", err)
		}
		grepRE = re
	}

	maxName := 0
	for _, n := range names {
		if len(n) > maxName {
			maxName = len(n)
		}
	}

	var wg sync.WaitGroup
	procs := make([]*exec.Cmd, 0, len(names))

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for i, n := range names {
		jArgs := []string{systemd.CurrentMode().Flag(),
			"-u", systemd.ServiceUnit(n),
			"-o", "short-iso",
			"-n", strconv.Itoa(logsLines),
			"--no-hostname",
		}
		if !logsNoFollow {
			jArgs = append(jArgs, "-f")
		}
		if logsSince != "" {
			jArgs = append(jArgs, "--since", logsSince)
		}
		c := exec.CommandContext(ctx, "journalctl", jArgs...)
		stdout, err := c.StdoutPipe()
		if err != nil {
			return err
		}
		c.Stderr = os.Stderr
		if err := c.Start(); err != nil {
			return fmt.Errorf("start journalctl for %s: %w", n, err)
		}
		procs = append(procs, c)
		col := prefixPalette[i%len(prefixPalette)]
		wg.Add(1)
		go func(name string, r io.Reader, col *color.Color) {
			defer wg.Done()
			pumpLogs(name, r, col, maxName, grepRE)
		}(n, stdout, col)
	}

	wg.Wait()
	for _, c := range procs {
		_ = c.Wait()
	}
	return nil
}

func pumpLogs(name string, r io.Reader, prefixCol *color.Color, padTo int, grepRE *regexp.Regexp) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	prefix := ""
	if !logsRaw {
		prefix = prefixCol.Sprint(ui.PadRight(name, padTo)) + ui.Dim.Sprint(" │ ")
	}
	for sc.Scan() {
		line := sc.Text()
		if grepRE != nil && !grepRE.MatchString(line) {
			continue
		}
		fmt.Println(prefix + colorizeLog(line))
	}
}

func colorizeLog(line string) string {
	// Detect leading ISO timestamp from journalctl short-iso and dim it.
	out := line
	if len(out) > 20 && (out[4] == '-' || out[4] == 'T') {
		// "2025-01-12T14:23:45+0000 host? service[pid]: message"
		// short-iso w/ --no-hostname: "2025-01-12T14:23:45+0000 service[pid]: message"
		idx := strings.Index(out, " ")
		if idx > 10 {
			ts := out[:idx]
			rest := out[idx:]
			// further dim the "service[pid]:" prefix
			tail := rest
			if c := strings.Index(rest, ": "); c > 0 && c < 80 {
				meta := rest[:c+1]
				msg := rest[c+1:]
				tail = ui.Dim.Sprint(meta) + colorizeLevel(msg)
			} else {
				tail = colorizeLevel(rest)
			}
			out = ui.Dim.Sprint(ts) + tail
			return out
		}
	}
	return colorizeLevel(out)
}

func colorizeLevel(s string) string {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "panic"), strings.Contains(low, "fatal"):
		return ui.Red.Sprint(s)
	case strings.Contains(low, " error") || strings.HasPrefix(low, "error") || strings.Contains(low, "[error]"):
		return ui.Red.Sprint(s)
	case strings.Contains(low, " warn") || strings.HasPrefix(low, "warn") || strings.Contains(low, "[warn]"):
		return ui.Yellow.Sprint(s)
	case strings.Contains(low, " debug") || strings.HasPrefix(low, "debug"):
		return ui.Dim.Sprint(s)
	default:
		return logLevelRE.ReplaceAllStringFunc(s, func(m string) string {
			low := strings.ToLower(m)
			switch low {
			case "error", "err", "fatal", "panic":
				return ui.Red.Sprint(m)
			case "warn", "warning":
				return ui.Yellow.Sprint(m)
			case "info":
				return ui.Cyan.Sprint(m)
			case "debug", "trace":
				return ui.Dim.Sprint(m)
			}
			return m
		})
	}
}
