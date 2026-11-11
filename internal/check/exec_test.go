package check_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/igorwulff/gochecks/internal/check"
)

const fakeTool = `package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	switch os.Args[1] {
	case "findings":
		fmt.Println("one")
		os.Exit(1)
	case "sleep":
		time.Sleep(10 * time.Second)
	}
}
`

var fakeBin string

// TestMain compiles a fake wrapped tool so the tests behave the same on every OS.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fake")
	if err != nil {
		panic(err)
	}

	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(fakeTool), 0o644); err != nil {
		panic(err)
	}

	fakeBin = filepath.Join(dir, "fake")
	if out, err := exec.Command("go", "build", "-o", fakeBin, src).CombinedOutput(); err != nil {
		panic(string(out))
	}

	code := m.Run()
	_ = os.RemoveAll(dir)

	os.Exit(code)
}

func fakeCheck(mode string) check.Check {
	return check.Check{
		Name:   "fake",
		Binary: fakeBin,
		Args:   func(check.Scope) []string { return []string{mode} },
		Parse: func(o check.Output) ([]check.Finding, error) {
			if o.ExitCode == 1 {
				return []check.Finding{{Check: "fake", Message: "one"}}, nil
			}

			return nil, nil
		},
	}
}

func TestExecRunner(t *testing.T) {
	r := check.ExecRunner{Timeout: 5 * time.Second}
	if res := r.Run(context.Background(), fakeCheck("ok"), check.Scope{Dir: t.TempDir()}); res.Status != check.StatusPassed {
		t.Errorf("ok: status = %v, err = %v", res.Status, res.Err)
	}

	if res := r.Run(context.Background(), fakeCheck("findings"), check.Scope{Dir: t.TempDir()}); res.Status != check.StatusFindings || len(res.Findings) != 1 {
		t.Errorf("findings: status = %v, findings = %v", res.Status, res.Findings)
	}
}

func TestExecRunnerTimeoutIsToolError(t *testing.T) {
	r := check.ExecRunner{Timeout: 200 * time.Millisecond}

	res := r.Run(context.Background(), fakeCheck("sleep"), check.Scope{Dir: t.TempDir()})
	if res.Status != check.StatusError || res.Err == nil {
		t.Errorf("status = %v, err = %v; want a tool error", res.Status, res.Err)
	}
}

func TestExecRunnerMissingTool(t *testing.T) {
	c := fakeCheck("ok")
	c.Binary, c.Install = "definitely-not-installed-xyz", "go install example.com/x@latest"
	res := check.ExecRunner{}.Run(context.Background(), c, check.Scope{Dir: t.TempDir()})

	var ni *check.ErrNotInstalled
	if !errors.As(res.Err, &ni) || res.Status != check.StatusError {
		t.Fatalf("status = %v, err = %v; want ErrNotInstalled", res.Status, res.Err)
	}
}
