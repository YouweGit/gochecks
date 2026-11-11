// Package tui renders live Check progress with Bubble Tea.
package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YouweGit/gochecks/internal/check"
	"github.com/YouweGit/gochecks/internal/report"
	"github.com/YouweGit/gochecks/internal/runner"
)

type doneMsg struct{ results []check.Result }

type tickMsg struct{}

type model struct {
	jobs     []runner.Job
	statuses []check.Status
	results  []check.Result
	started  []time.Time
	elapsed  []time.Duration
	frame    int
	done     bool
	cancel   func()
}

func (m model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runner.Update:
		m.statuses[msg.Index] = msg.Status
		switch msg.Status {
		case check.StatusRunning:
			m.started[msg.Index] = time.Now()
		case check.StatusPassed, check.StatusFindings, check.StatusError:
			m.elapsed[msg.Index] = time.Since(m.started[msg.Index])
		}
	case tickMsg:
		m.frame++
		return m, tick()
	case doneMsg:
		m.results, m.done = msg.results, true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.cancel()
		}
	}

	return m, nil
}

var (
	spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	cyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	green   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dim     = lipgloss.NewStyle().Faint(true)
)

func (m model) View() string {
	if m.done {
		return ""
	}

	width := 0
	for _, j := range m.jobs {
		width = max(width, len(j.Check.Name))
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("gochecks") + dim.Render("  q to cancel") + "\n\n")

	for i, j := range m.jobs {
		icon, took := dim.Render("·"), ""

		switch m.statuses[i] {
		case check.StatusRunning:
			icon = cyan.Render(spinner[m.frame%len(spinner)])
			took = report.FormatDuration(time.Since(m.started[i]))
		case check.StatusPassed:
			icon, took = green.Render("✓"), report.FormatDuration(m.elapsed[i])
		case check.StatusFindings:
			icon, took = red.Render("✗"), report.FormatDuration(m.elapsed[i])
		case check.StatusError:
			icon, took = yellow.Render("!"), report.FormatDuration(m.elapsed[i])
		}

		fmt.Fprintf(&b, " %s %-*s  %s  %s\n", icon, width, j.Check.Name, dim.Render(filepath.Base(j.Scope.Dir)), dim.Render(took))
	}

	return b.String()
}

// Run shows live progress while start executes the jobs, then returns the
// results. start must call send for every status change. cancel is invoked
// when the user quits early.
func Run(jobs []runner.Job, cancel func(), start func(send func(runner.Update)) []check.Result) ([]check.Result, error) {
	m := model{jobs: jobs, statuses: make([]check.Status, len(jobs)), started: make([]time.Time, len(jobs)), elapsed: make([]time.Duration, len(jobs)), cancel: cancel}
	p := tea.NewProgram(m)

	go func() {
		results := start(func(u runner.Update) { p.Send(u) })
		p.Send(doneMsg{results: results})
	}()

	final, err := p.Run()
	if err != nil {
		return nil, err
	}

	return final.(model).results, nil
}
