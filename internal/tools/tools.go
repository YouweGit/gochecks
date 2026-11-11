// Package tools defines the Checks that wrap external Go tooling.
package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/igorwulff/gochecks/internal/check"
)

// Default returns every built-in Check in a stable order.
func Default() []check.Check {
	return []check.Check{Gofmt(), Vet(), GolangCILint(), Govulncheck(), Gosec(), GoTest()}
}

var locRe = regexp.MustCompile(`^(?:vet: )?(\S[^:]*):(\d+)(?::\d+)?: (.+)$`)

func tail(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		s = "..." + s[len(s)-500:]
	}

	return s
}

// Gofmt wraps `gofmt -l`, reporting files that are not formatted.
func Gofmt() check.Check {
	return check.Check{
		Name:   "gofmt",
		Binary: "gofmt",
		Args: func(s check.Scope) []string {
			args := []string{"-l"}
			if s.All || len(s.Packages) == 0 {
				return append(args, ".")
			}

			return append(args, s.Packages...)
		},
		Parse: func(o check.Output) ([]check.Finding, error) {
			var out []check.Finding

			for _, line := range strings.Split(string(o.Stdout), "\n") {
				if f := strings.TrimSpace(line); f != "" {
					out = append(out, check.Finding{Check: "gofmt", File: f, Message: "file is not gofmt-formatted (run gofmt -w)"})
				}
			}

			for _, line := range strings.Split(string(o.Stderr), "\n") {
				if m := locRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
					n, _ := strconv.Atoi(m[2])
					out = append(out, check.Finding{Check: "gofmt", File: m[1], Line: n, Message: m[3]})
				}
			}

			if o.ExitCode != 0 && len(out) == 0 {
				return nil, fmt.Errorf("exit code %d: %s", o.ExitCode, tail(o.Stderr))
			}

			return out, nil
		},
	}
}

// Vet wraps `go vet`.
func Vet() check.Check {
	return check.Check{
		Name:   "vet",
		Binary: "go",
		Args: func(s check.Scope) []string {
			return append([]string{"vet"}, s.Patterns()...)
		},
		Parse: func(o check.Output) ([]check.Finding, error) {
			var out []check.Finding

			for _, line := range strings.Split(string(o.Stderr), "\n") {
				if m := locRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
					n, _ := strconv.Atoi(m[2])
					out = append(out, check.Finding{Check: "vet", File: m[1], Line: n, Message: m[3]})
				}
			}

			if o.ExitCode != 0 && len(out) == 0 {
				return nil, fmt.Errorf("exit code %d: %s", o.ExitCode, tail(o.Stderr))
			}

			return out, nil
		},
	}
}

// GolangCILint wraps golangci-lint v2.
func GolangCILint() check.Check {
	return check.Check{
		Name:    "golangci-lint",
		Binary:  "golangci-lint",
		Install: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest",
		Args: func(s check.Scope) []string {
			return append([]string{"run", "--output.json.path=stdout", "--output.text.path="}, s.Patterns()...)
		},
		Parse: func(o check.Output) ([]check.Finding, error) {
			var rep struct {
				Issues []struct {
					FromLinter string
					Text       string
					Pos        struct {
						Filename string
						Line     int
					}
				}
			}
			// Decode only the first JSON value; golangci-lint may print a summary after it.
			if err := json.NewDecoder(bytes.NewReader(o.Stdout)).Decode(&rep); err != nil {
				if o.ExitCode == 0 && len(bytes.TrimSpace(o.Stdout)) == 0 {
					return nil, nil
				}

				return nil, fmt.Errorf("exit code %d: %s", o.ExitCode, tail(append(o.Stderr, o.Stdout...)))
			}

			var out []check.Finding
			for _, i := range rep.Issues {
				out = append(out, check.Finding{
					Check: "golangci-lint", File: i.Pos.Filename, Line: i.Pos.Line,
					Message: fmt.Sprintf("%s: %s", i.FromLinter, i.Text),
				})
			}

			return out, nil
		},
	}
}

// Govulncheck wraps govulncheck; it always scans the whole module.
func Govulncheck() check.Check {
	return check.Check{
		Name:        "govulncheck",
		Binary:      "govulncheck",
		Install:     "go install golang.org/x/vuln/cmd/govulncheck@latest",
		WholeModule: true,
		Args: func(check.Scope) []string {
			return []string{"-format", "json", "./..."}
		},
		Parse: func(o check.Output) ([]check.Finding, error) {
			dec := json.NewDecoder(bytes.NewReader(o.Stdout))

			var out []check.Finding

			seen := map[string]bool{}

			for {
				var msg struct {
					Finding *struct {
						OSV   string `json:"osv"`
						Trace []struct {
							Module   string `json:"module"`
							Version  string `json:"version"`
							Function string `json:"function"`
						} `json:"trace"`
					} `json:"finding"`
				}
				if err := dec.Decode(&msg); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					return nil, fmt.Errorf("decoding output: %w", err)
				}

				f := msg.Finding
				// Only findings with a function in the trace are reachable from the code.
				if f == nil || len(f.Trace) == 0 || f.Trace[0].Function == "" || seen[f.OSV] {
					continue
				}

				seen[f.OSV] = true
				out = append(out, check.Finding{
					Check:   "govulncheck",
					Message: fmt.Sprintf("%s in %s@%s", f.OSV, f.Trace[0].Module, f.Trace[0].Version),
				})
			}

			if o.ExitCode != 0 && o.ExitCode != 3 && len(out) == 0 {
				return nil, fmt.Errorf("exit code %d: %s", o.ExitCode, tail(o.Stderr))
			}

			return out, nil
		},
	}
}

// Gosec wraps gosec.
func Gosec() check.Check {
	return check.Check{
		Name:    "gosec",
		Binary:  "gosec",
		Install: "go install github.com/securego/gosec/v2/cmd/gosec@latest",
		Args: func(s check.Scope) []string {
			return append([]string{"-fmt=json", "-quiet", "-no-fail"}, s.Patterns()...)
		},
		Parse: func(o check.Output) ([]check.Finding, error) {
			var rep struct {
				Issues []struct {
					RuleID  string `json:"rule_id"`
					Details string `json:"details"`
					File    string `json:"file"`
					Line    string `json:"line"`
				}
			}
			// With -quiet, gosec prints nothing when there are no issues.
			if len(bytes.TrimSpace(o.Stdout)) == 0 && o.ExitCode == 0 {
				return nil, nil
			}

			if err := json.Unmarshal(o.Stdout, &rep); err != nil {
				return nil, fmt.Errorf("exit code %d: %s", o.ExitCode, tail(o.Stderr))
			}

			var out []check.Finding

			for _, i := range rep.Issues {
				line, _, _ := strings.Cut(i.Line, "-")
				n, _ := strconv.Atoi(line)
				out = append(out, check.Finding{Check: "gosec", File: i.File, Line: n, Message: i.RuleID + ": " + i.Details})
			}

			return out, nil
		},
	}
}

// GoTest wraps `go test -race`; it only runs for `run --all`.
func GoTest() check.Check {
	return check.Check{
		Name:    "test",
		Binary:  "go",
		AllOnly: true,
		Args: func(s check.Scope) []string {
			return append([]string{"test", "-race", "-json"}, s.Patterns()...)
		},
		Parse: func(o check.Output) ([]check.Finding, error) {
			dec := json.NewDecoder(bytes.NewReader(o.Stdout))

			var out []check.Finding

			failedPkgs := map[string]bool{}
			pkgFailed := map[string]bool{}

			for {
				var ev struct {
					Action  string
					Package string
					Test    string
				}
				if err := dec.Decode(&ev); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					return nil, fmt.Errorf("decoding output: %w", err)
				}

				if ev.Action != "fail" {
					continue
				}

				if ev.Test != "" {
					failedPkgs[ev.Package] = true
					out = append(out, check.Finding{Check: "test", Message: fmt.Sprintf("%s: %s failed", ev.Package, ev.Test)})
				} else {
					pkgFailed[ev.Package] = true
				}
			}
			// Packages that failed without a failing test (build errors, races, panics).
			for pkg := range pkgFailed {
				if !failedPkgs[pkg] {
					out = append(out, check.Finding{Check: "test", Message: pkg + ": package failed"})
				}
			}

			if o.ExitCode != 0 && len(out) == 0 {
				return nil, fmt.Errorf("exit code %d: %s", o.ExitCode, tail(o.Stderr))
			}

			return out, nil
		},
	}
}
