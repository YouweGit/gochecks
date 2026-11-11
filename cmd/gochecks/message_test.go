package main

import (
	"strings"
	"testing"
)

func TestNothingToCheck(t *testing.T) {
	wd := "/repo"

	tests := []struct {
		name string
		c    changeSet
		want []string
	}{
		{"clean tree", changeSet{}, []string{"no staged changes", "working tree is clean", "--all"}},
		{
			"unstaged go file",
			changeSet{Unstaged: []string{"/repo/a/x.go"}},
			[]string{"no staged changes", "1 unstaged", "a/x.go", "git add"},
		},
		{
			"staged non-go",
			changeSet{Staged: []string{"/repo/README.md"}},
			[]string{"1 staged file(s), but none are .go"},
		},
		{
			"all disabled",
			changeSet{Staged: []string{"/repo/x.go"}, AllDisabled: true},
			[]string{"disabled in .gochecks.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nothingToCheck(tt.c, wd)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in:\n%s", w, got)
				}
			}
		})
	}
}
