package ui

import (
	"fmt"
	"io"
	"strings"
)

type Field struct {
	Label string
	Value string
}

func RenderCard(w io.Writer, title string, fields []Field) {
	maxLabel := 0
	for _, f := range fields {
		if l := visibleLen(f.Label); l > maxLabel {
			maxLabel = l
		}
	}
	if title != "" {
		fmt.Fprintln(w, Header.Sprint(title))
	}
	for _, f := range fields {
		label := Label.Sprint(PadRight(f.Label, maxLabel))
		fmt.Fprintf(w, "  %s  %s\n", label, f.Value)
	}
}

func RenderCards(w io.Writer, cards []Card) {
	for i, c := range cards {
		if i > 0 {
			fmt.Fprintln(w)
		}
		RenderCard(w, c.Title, c.Fields)
	}
}

type Card struct {
	Title  string
	Fields []Field
}

func Hr(width int) string {
	if width <= 0 {
		width = TermWidth()
	}
	return Dim.Sprint(strings.Repeat("─", width))
}
