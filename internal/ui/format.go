package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visibleLen(s string) int {
	return len(ansiRE.ReplaceAllString(s, ""))
}

func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func Bytes(b uint64) string {
	if b == 0 {
		return Dim.Sprint("—")
	}
	const u = 1024
	if b < u {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func Duration(d time.Duration) string {
	if d <= 0 {
		return Dim.Sprint("—")
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours()) / 24
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	return fmt.Sprintf("%dy", months/12)
}

func Percent(p float64) string {
	if p < 0 {
		p = 0
	}
	return fmt.Sprintf("%.1f%%", p)
}

func PadRight(s string, n int) string {
	pad := n - visibleLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func PadLeft(s string, n int) string {
	pad := n - visibleLen(s)
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

func Truncate(s string, n int) string {
	if visibleLen(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	plain := StripANSI(s)
	return plain[:n-1] + "…"
}
