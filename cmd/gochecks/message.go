package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/igorwulff/gochecks/internal/project"
)

// changeSet describes the working tree as seen by `run`.
type changeSet struct {
	Staged, Unstaged, Untracked []string
	// AllDisabled is set when Checks apply but every one is disabled in config.
	AllDisabled bool
}

// nothingToCheck explains why a staged run had no work; wd makes paths relative.
func nothingToCheck(c changeSet, wd string) string {
	var b strings.Builder

	switch {
	case c.AllDisabled:
		b.WriteString("Nothing to check: every applicable Check is disabled in .gochecks.yml.\n")
	case len(c.Staged) == 0:
		b.WriteString("Nothing to check: no staged changes.\n")
	default:
		fmt.Fprintf(&b, "Nothing to check: %d staged file(s), but none are .go files or go.mod/go.sum.\n", len(c.Staged))
	}

	b.WriteString("`run` only checks what is staged (git diff --cached), not the working tree.\n")

	if pending := relevantFiles(c.Unstaged, c.Untracked); len(pending) > 0 {
		fmt.Fprintf(&b, "\nNot checked: %d unstaged and %d untracked file(s), %d of them Go-related:\n",
			len(c.Unstaged), len(c.Untracked), len(pending))

		for i, f := range pending {
			if i == 5 {
				fmt.Fprintf(&b, "  … and %d more\n", len(pending)-i)

				break
			}

			fmt.Fprintf(&b, "  %s\n", relTo(wd, f))
		}

		b.WriteString("\nStage them with `git add` to check them, or run `gochecks run --all` to check the whole module.\n")
	} else if len(c.Unstaged)+len(c.Untracked) > 0 {
		fmt.Fprintf(&b, "\nThere are %d unstaged and %d untracked file(s), but none are Go-related.\n", len(c.Unstaged), len(c.Untracked))
	} else if len(c.Staged) == 0 {
		b.WriteString("\nThe working tree is clean. Run `gochecks run --all` to check the whole module.\n")
	}

	return b.String()
}

func relevantFiles(lists ...[]string) []string {
	var out []string

	for _, l := range lists {
		for _, f := range l {
			if project.Relevant(f) {
				out = append(out, f)
			}
		}
	}

	return out
}

func relTo(wd, path string) string {
	if rel, err := filepath.Rel(wd, path); err == nil {
		return filepath.ToSlash(rel)
	}

	return path
}
