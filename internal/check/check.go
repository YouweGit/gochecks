// Package check defines the Check and Finding types shared by every other package.
package check

import (
	"context"
	"time"
)

// Finding is a single issue reported by a Check.
type Finding struct {
	Check   string `json:"check"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

// Status is the outcome of running a Check.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusPassed
	StatusFindings
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusPassed:
		return "passed"
	case StatusFindings:
		return "findings"
	case StatusError:
		return "error"
	default:
		return "pending"
	}
}

// Scope tells a Check what to analyse.
type Scope struct {
	// Dir is the module root the Check runs in.
	Dir string
	// Packages are package patterns relative to Dir (e.g. "./internal/foo").
	// Empty with All set means the whole module.
	Packages []string
	All      bool
}

// Patterns returns the package patterns to pass to Go tooling.
func (s Scope) Patterns() []string {
	if s.All || len(s.Packages) == 0 {
		return []string{"./..."}
	}

	return s.Packages
}

// Check is one invocation of a wrapped tool against a module.
type Check struct {
	Name string
	// Binary is the executable looked up on PATH.
	Binary string
	// Install is the command that installs Binary; empty when it ships with Go.
	Install string
	// AllOnly Checks run only for `run --all`.
	AllOnly bool
	// WholeModule Checks always analyse the whole module, even for staged runs.
	WholeModule bool
	// Mutates Checks change files and therefore run serially.
	Mutates bool
	Args    func(Scope) []string
	// Parse turns tool output into Findings. A non-nil error is a tool error.
	Parse func(out Output) ([]Finding, error)
}

// Output is the captured result of a subprocess.
type Output struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Result is the outcome of one Check against one module.
type Result struct {
	Check    string
	Dir      string
	Status   Status
	Findings []Finding
	Err      error
	Duration time.Duration
}

// Runner executes a Check; it is the seam tests use to avoid real subprocesses.
type Runner interface {
	Run(ctx context.Context, c Check, s Scope) Result
}
