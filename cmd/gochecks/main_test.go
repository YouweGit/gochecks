package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecuteHelpAndUsageErrors(t *testing.T) {
	tests := []struct {
		args     []string
		wantCode int
		wantOut  string
	}{
		{nil, 0, "Commands:"},
		{[]string{"help"}, 0, "Commands:"},
		{[]string{"--help"}, 0, "Commands:"},
		{[]string{"help", "run"}, 0, "-no-tui"},
		{[]string{"run", "-h"}, 0, "-no-tui"},
		{[]string{"init", "--help"}, 0, "pre-commit hook"},
		{[]string{"completion", "bash"}, 0, "complete -F"},
		{[]string{"completion", "tcsh"}, 2, ""},
		{[]string{"help", "nope"}, 2, ""},
		{[]string{"bogus"}, 2, ""},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer

		if got := execute(tt.args, &stdout, &stderr); got != tt.wantCode {
			t.Errorf("execute(%v) = %d, want %d", tt.args, got, tt.wantCode)
		}

		if !strings.Contains(stdout.String(), tt.wantOut) {
			t.Errorf("execute(%v) stdout = %q, want it to contain %q", tt.args, stdout.String(), tt.wantOut)
		}
	}
}
