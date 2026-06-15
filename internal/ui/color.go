package ui

import (
	"github.com/fatih/color"
)

var (
	Green   = color.New(color.FgGreen)
	Red     = color.New(color.FgRed)
	Yellow  = color.New(color.FgYellow)
	Cyan    = color.New(color.FgCyan)
	Magenta = color.New(color.FgMagenta)
	Blue    = color.New(color.FgBlue)
	White   = color.New(color.FgWhite)
	Bold    = color.New(color.Bold)
	Dim     = color.New(color.Faint)
	Header  = color.New(color.Bold, color.FgCyan)
	Label   = color.New(color.Faint)
)

func StatusColor(active string) *color.Color {
	switch active {
	case "active":
		return Green
	case "failed":
		return Red
	case "inactive", "activating", "reloading", "deactivating":
		return Yellow
	default:
		return Dim
	}
}

func StatusDot(active, sub string) string {
	c := StatusColor(active)
	return c.Sprint("●")
}

func StatusLabel(active, sub string) string {
	c := StatusColor(active)
	switch active {
	case "active":
		return c.Sprint("online")
	case "inactive":
		return c.Sprint("stopped")
	case "":
		return Dim.Sprint("unknown")
	default:
		return c.Sprint(active)
	}
}
