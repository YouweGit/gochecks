package check

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ExecRunner runs Checks as real subprocesses found via PATH.
type ExecRunner struct {
	Timeout time.Duration
}

// ErrNotInstalled is returned when a wrapped tool is not on PATH.
type ErrNotInstalled struct{ Check Check }

func (e *ErrNotInstalled) Error() string {
	if e.Check.Install == "" {
		return fmt.Sprintf("%s: %q not found on PATH", e.Check.Name, e.Check.Binary)
	}

	return fmt.Sprintf("%s: %q not found on PATH; install with: %s", e.Check.Name, e.Check.Binary, e.Check.Install)
}

func (r ExecRunner) Run(ctx context.Context, c Check, s Scope) Result {
	start := time.Now()
	res := Result{Check: c.Name, Dir: s.Dir}
	finish := func() Result {
		res.Duration = time.Since(start)
		return res
	}

	path, err := exec.LookPath(c.Binary)
	if err != nil {
		res.Status, res.Err = StatusError, &ErrNotInstalled{Check: c}
		return finish()
	}

	if r.Timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, path, c.Args(s)...) // #nosec G204 -- runs the built-in wrapped tools only
	cmd.Dir = s.Dir
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	if ctxErr := ctx.Err(); ctxErr != nil {
		res.Status = StatusError
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			res.Err = fmt.Errorf("%s: timed out after %s", c.Name, r.Timeout)
		} else {
			res.Err = fmt.Errorf("%s: %w", c.Name, ctxErr)
		}

		return finish()
	}

	out := Output{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}

	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			res.Status, res.Err = StatusError, fmt.Errorf("%s: %w", c.Name, runErr)
			return finish()
		}

		out.ExitCode = exitErr.ExitCode()
	}

	findings, err := c.Parse(out)
	switch {
	case err != nil:
		res.Status, res.Err = StatusError, fmt.Errorf("%s: %w", c.Name, err)
	case len(findings) > 0:
		res.Status, res.Findings = StatusFindings, findings
	default:
		res.Status = StatusPassed
	}

	return finish()
}
