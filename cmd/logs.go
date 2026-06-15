package cmd

import (
	"bufio"
	"encoding/json"
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
	"time"

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
	logLevelRE    = regexp.MustCompile(`(?i)\b(error|err|fatal|panic|warn|warning|info|debug|trace)\b`)
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
		// -o json is the only output mode that preserves ANSI escape bytes
		// in MESSAGE — all `short*` formats sanitise control characters even
		// when journalctl is writing to a TTY. We pay for it with a JSON
		// parse per line, which is fine at human-scale log rates.
		jArgs := []string{systemd.CurrentMode().Flag(),
			"-u", systemd.ServiceUnit(n),
			"-o", "json",
			"-n", strconv.Itoa(logsLines),
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

type journalEntry struct {
	Realtime string          `json:"__REALTIME_TIMESTAMP"`
	Message  json.RawMessage `json:"MESSAGE"`
}

func pumpLogs(name string, r io.Reader, prefixCol *color.Color, padTo int, grepRE *regexp.Regexp) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	prefix := ""
	if !logsRaw {
		prefix = prefixCol.Sprint(ui.PadRight(name, padTo)) + ui.Dim.Sprint(" │ ")
	}
	for sc.Scan() {
		line := sc.Bytes()
		var e journalEntry
		if err := json.Unmarshal(line, &e); err != nil {
			// Non-JSON line (rare; could be a journalctl info message).
			text := string(line)
			if grepRE != nil && !grepRE.MatchString(text) {
				continue
			}
			fmt.Println(prefix + text)
			continue
		}
		msg := decodeJournalMessage(e.Message)
		if grepRE != nil && !grepRE.MatchString(msg) {
			continue
		}
		fmt.Println(prefix + formatTimestamp(e.Realtime) + " " + colorizeLevel(msg))
	}
}

// decodeJournalMessage handles journalctl's two MESSAGE representations:
// a normal JSON string when the bytes are clean UTF-8 without control chars,
// or an array of byte values (e.g. [27, 91, 51, 52, 109, …]) whenever any
// byte trips journalctl's "non-printable" check — which includes every ANSI
// escape sequence.
func decodeJournalMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	}
	var nums []int
	if err := json.Unmarshal(raw, &nums); err != nil {
		return ""
	}
	b := make([]byte, len(nums))
	for i, n := range nums {
		b[i] = byte(n)
	}
	return string(b)
}

func formatTimestamp(realtime string) string {
	us, err := strconv.ParseInt(realtime, 10, 64)
	if err != nil || us == 0 {
		return ui.Dim.Sprint("                    ")
	}
	return ui.Dim.Sprint(time.UnixMicro(us).Local().Format("2006-01-02 15:04:05"))
}

func colorizeLevel(s string) string {
	// If the line already carries ANSI from the service itself, leave it
	// untouched — otherwise our regex would wrap matches with red/reset and
	// truncate the service's own color spans.
	if strings.Contains(s, "\x1b[") {
		return s
	}
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
