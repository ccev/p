package ui

import (
	"io"
	"sort"
	"strings"
)

type Align int

const (
	AlignLeft Align = iota
	AlignRight
)

type Column struct {
	Header   string
	Align    Align
	Priority int // 0 = always show; higher = drop first on narrow terminals
	MaxWidth int // 0 = no limit
}

type Table struct {
	Columns []Column
	Rows    [][]string
}

// Render writes a borderless padded table similar to kubectl/docker output.
// On terminals narrower than the minimum row width, columns with the highest
// Priority are dropped first. Always-required columns (Priority 0) are kept.
func (t *Table) Render(w io.Writer) {
	if len(t.Columns) == 0 {
		return
	}
	termW := TermWidth()
	visible := t.fitColumns(termW)

	widths := make([]int, len(t.Columns))
	for _, i := range visible {
		widths[i] = visibleLen(t.Columns[i].Header)
	}
	for _, row := range t.Rows {
		for _, i := range visible {
			if i >= len(row) {
				continue
			}
			l := visibleLen(row[i])
			if t.Columns[i].MaxWidth > 0 && l > t.Columns[i].MaxWidth {
				l = t.Columns[i].MaxWidth
			}
			if l > widths[i] {
				widths[i] = l
			}
		}
	}
	// Shrink last column to fit if still overflowing.
	used := 0
	for _, i := range visible {
		used += widths[i] + 2
	}
	if used > termW && len(visible) > 0 {
		last := visible[len(visible)-1]
		excess := used - termW
		if widths[last]-excess >= 4 {
			widths[last] -= excess
		}
	}

	// Header
	var hb strings.Builder
	for n, i := range visible {
		cell := Header.Sprint(t.Columns[i].Header)
		if t.Columns[i].Align == AlignRight {
			cell = PadLeft(cell, widths[i])
		} else {
			cell = PadRight(cell, widths[i])
		}
		hb.WriteString(cell)
		if n < len(visible)-1 {
			hb.WriteString("  ")
		}
	}
	hb.WriteString("\n")
	io.WriteString(w, hb.String())

	// Rows
	for _, row := range t.Rows {
		var rb strings.Builder
		for n, i := range visible {
			var cell string
			if i < len(row) {
				cell = row[i]
			}
			cell = Truncate(cell, widths[i])
			if t.Columns[i].Align == AlignRight {
				cell = PadLeft(cell, widths[i])
			} else {
				cell = PadRight(cell, widths[i])
			}
			rb.WriteString(cell)
			if n < len(visible)-1 {
				rb.WriteString("  ")
			}
		}
		rb.WriteString("\n")
		io.WriteString(w, rb.String())
	}
}

func (t *Table) fitColumns(termW int) []int {
	indices := make([]int, len(t.Columns))
	for i := range indices {
		indices[i] = i
	}
	measure := func(idxs []int) int {
		widths := make(map[int]int)
		for _, i := range idxs {
			widths[i] = visibleLen(t.Columns[i].Header)
		}
		for _, row := range t.Rows {
			for _, i := range idxs {
				if i >= len(row) {
					continue
				}
				l := visibleLen(row[i])
				if t.Columns[i].MaxWidth > 0 && l > t.Columns[i].MaxWidth {
					l = t.Columns[i].MaxWidth
				}
				if l > widths[i] {
					widths[i] = l
				}
			}
		}
		total := 0
		for _, w := range widths {
			total += w + 2
		}
		return total
	}
	for measure(indices) > termW && len(indices) > 1 {
		// Find highest-priority column to drop.
		dropAt := -1
		for j, i := range indices {
			if t.Columns[i].Priority == 0 {
				continue
			}
			if dropAt == -1 || t.Columns[i].Priority > t.Columns[indices[dropAt]].Priority {
				dropAt = j
			}
		}
		if dropAt == -1 {
			break
		}
		indices = append(indices[:dropAt], indices[dropAt+1:]...)
	}
	sort.Ints(indices)
	return indices
}
