// Package report renders Check results for humans and machines.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/igorwulff/gochecks/internal/check"
)

// Plain writes a human-readable report; colour is used only when w is a terminal.
func Plain(w io.Writer, results []check.Result) {
	r := lipgloss.NewRenderer(w)
	green := r.NewStyle().Foreground(lipgloss.Color("2"))
	red := r.NewStyle().Foreground(lipgloss.Color("1"))
	yellow := r.NewStyle().Foreground(lipgloss.Color("3"))
	dim := r.NewStyle().Faint(true)
	bold := r.NewStyle().Bold(true)

	width, dirs := 0, map[string]bool{}
	for _, res := range results {
		width = max(width, len(res.Check))
		dirs[res.Dir] = true
	}

	var passed, failed, errored, findings int

	for _, res := range results {
		icon, style := green.Render("✓"), green
		note := ""

		switch res.Status {
		case check.StatusPassed:
			passed++
		case check.StatusFindings:
			failed++
			findings += len(res.Findings)
			icon, style = red.Render("✗"), red
			note = fmt.Sprintf("%d finding(s)", len(res.Findings))
		case check.StatusError:
			errored++
			icon, style = yellow.Render("!"), yellow
			note = "tool error"
		}

		line := fmt.Sprintf(" %s %s", icon, style.Render(fmt.Sprintf("%-*s", width, res.Check)))
		if len(dirs) > 1 {
			line += "  " + dim.Render(filepath.Base(res.Dir))
		}

		line += "  " + dim.Render(FormatDuration(res.Duration))
		if note != "" {
			line += "  " + style.Render(note)
		}

		fmt.Fprintln(w, line)

		for _, f := range res.Findings {
			fmt.Fprintf(w, "     %s\n", formatFinding(f, dim))
		}

		if res.Err != nil {
			fmt.Fprintf(w, "     %s\n", yellow.Render(res.Err.Error()))
		}
	}

	fmt.Fprintln(w)

	switch failed + errored {
	case 0:
		fmt.Fprintln(w, bold.Render(green.Render(fmt.Sprintf("All %d checks passed", passed))))
	default:
		parts := []string{fmt.Sprintf("%d passed", passed)}
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%d failed (%d findings)", failed, findings))
		}

		if errored > 0 {
			parts = append(parts, fmt.Sprintf("%d tool error(s)", errored))
		}

		fmt.Fprintln(w, bold.Render(red.Render(strings.Join(parts, " · "))))
	}
}

// FormatDuration renders a duration compactly (e.g. 85ms, 1.4s).
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatFinding(f check.Finding, dim lipgloss.Style) string {
	switch {
	case f.File != "" && f.Line > 0:
		return fmt.Sprintf("%s %s", dim.Render(fmt.Sprintf("%s:%d", f.File, f.Line)), f.Message)
	case f.File != "":
		return fmt.Sprintf("%s %s", dim.Render(f.File), f.Message)
	default:
		return f.Message
	}
}

// JSON writes aggregated Findings and tool errors as a single document.
func JSON(w io.Writer, results []check.Result) error {
	type jsonResult struct {
		Check    string          `json:"check"`
		Dir      string          `json:"dir"`
		Status   string          `json:"status"`
		Error    string          `json:"error,omitempty"`
		Findings []check.Finding `json:"findings"`
	}

	doc := struct {
		Results []jsonResult `json:"results"`
	}{Results: make([]jsonResult, 0, len(results))}
	for _, r := range results {
		jr := jsonResult{Check: r.Check, Dir: r.Dir, Status: r.Status.String(), Findings: r.Findings}
		if jr.Findings == nil {
			jr.Findings = []check.Finding{}
		}

		if r.Err != nil {
			jr.Error = r.Err.Error()
		}

		doc.Results = append(doc.Results, jr)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(doc)
}
