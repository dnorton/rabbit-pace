package main

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Report is what a command prints: a few summary lines and an optional
// table. It can be rendered for the terminal or as Markdown.
type Report struct {
	Summary []Field
	Table   *Table
}

// Field is one "Label: value" summary line.
type Field struct {
	Label, Value string
}

// Table is a table of right-aligned values.
type Table struct {
	Headers []string
	Rows    []Row
	// TrendCols are the columns coloured by each row's Trend (the pace
	// and speed columns).
	TrendCols []int
}

// Row is one table row.
type Row struct {
	Cells []string
	Half  bool // the halfway row, highlighted
	Trend int  // -1 faster than target, +1 slower, 0 on target
}

// Colours use the terminal's 16 basic ANSI colours, so they follow the
// user's own theme and stay readable on light and dark backgrounds.
var (
	labelStyle  = lipgloss.NewStyle().Faint(true)
	valueStyle  = lipgloss.NewStyle().Bold(true)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Padding(0, 1).Align(lipgloss.Right)
	cellStyle   = lipgloss.NewStyle().Padding(0, 1).Align(lipgloss.Right)
	borderStyle = lipgloss.NewStyle().Faint(true)
	fasterColor = lipgloss.Color("2") // green
	slowerColor = lipgloss.Color("3") // yellow
	halfColor   = lipgloss.Color("5") // magenta
)

// WriteStyled renders r for a terminal. Colours are written as ANSI codes;
// main wraps stdout so they're dropped when it isn't a terminal or
// NO_COLOR is set.
func (r Report) WriteStyled(w io.Writer) error {
	width := 0
	for _, f := range r.Summary {
		width = max(width, len(f.Label))
	}
	var b strings.Builder
	for _, f := range r.Summary {
		label := fmt.Sprintf("%-*s", width, f.Label)
		fmt.Fprintf(&b, "%s  %s\n", labelStyle.Render(label), valueStyle.Render(f.Value))
	}

	if t := r.Table; t != nil {
		trendCol := make(map[int]bool)
		for _, c := range t.TrendCols {
			trendCol[c] = true
		}
		rows := make([][]string, len(t.Rows))
		for i, row := range t.Rows {
			rows[i] = row.Cells
		}
		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(borderStyle).
			Headers(t.Headers...).
			Rows(rows...).
			StyleFunc(func(i, col int) lipgloss.Style {
				if i == table.HeaderRow {
					return headerStyle
				}
				row, style := t.Rows[i], cellStyle
				if trendCol[col] {
					switch {
					case row.Trend < 0:
						style = style.Foreground(fasterColor)
					case row.Trend > 0:
						style = style.Foreground(slowerColor)
					}
				}
				if row.Half {
					style = style.Bold(true)
					if col == 0 {
						style = style.Foreground(halfColor)
					}
				}
				return style
			})
		fmt.Fprintf(&b, "\n%s\n", tbl.Render())
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// WriteMarkdown renders r as Markdown: the summary as a bulleted list and
// the table as a pipe table, ready to paste into Obsidian or a README.
func (r Report) WriteMarkdown(w io.Writer) error {
	var b strings.Builder
	for _, f := range r.Summary {
		fmt.Fprintf(&b, "- **%s:** %s\n", f.Label, f.Value)
	}

	if t := r.Table; t != nil {
		b.WriteString("\n| " + strings.Join(t.Headers, " | ") + " |\n|")
		for range t.Headers {
			b.WriteString("---:|")
		}
		b.WriteString("\n")
		for _, row := range t.Rows {
			cells := append([]string(nil), row.Cells...)
			if row.Half {
				cells[0] = "**" + cells[0] + "**"
			}
			b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// write renders r as Markdown if markdown is set, otherwise for the
// terminal.
func (r Report) write(w io.Writer, markdown bool) error {
	if markdown {
		return r.WriteMarkdown(w)
	}
	return r.WriteStyled(w)
}
