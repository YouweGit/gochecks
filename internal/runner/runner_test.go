package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YouweGit/gochecks/internal/check"
	"github.com/YouweGit/gochecks/internal/project"
)

type fakeRunner struct {
	running, peak atomic.Int32
	order         chan string
}

func (f *fakeRunner) Run(_ context.Context, c check.Check, s check.Scope) check.Result {
	n := f.running.Add(1)
	for {
		p := f.peak.Load()
		if n <= p || f.peak.CompareAndSwap(p, n) {
			break
		}
	}

	time.Sleep(20 * time.Millisecond)
	f.running.Add(-1)

	if f.order != nil {
		f.order <- c.Name
	}

	return check.Result{Check: c.Name, Dir: s.Dir, Status: check.StatusPassed}
}

func TestRunBoundsConcurrency(t *testing.T) {
	var jobs []Job
	for range 8 {
		jobs = append(jobs, Job{Check: check.Check{Name: "c"}})
	}

	f := &fakeRunner{}
	Run(context.Background(), f, jobs, 3, nil)

	if got := f.peak.Load(); got > 3 || got < 2 {
		t.Errorf("peak concurrency = %d, want 2..3", got)
	}
}

func TestRunMutatingChecksRunLast(t *testing.T) {
	jobs := []Job{
		{Check: check.Check{Name: "fmt", Mutates: true}},
		{Check: check.Check{Name: "a"}},
		{Check: check.Check{Name: "b"}},
	}
	f := &fakeRunner{order: make(chan string, 3)}
	Run(context.Background(), f, jobs, 4, nil)
	close(f.order)

	var names []string
	for n := range f.order {
		names = append(names, n)
	}

	if names[len(names)-1] != "fmt" {
		t.Errorf("order = %v, want fmt last", names)
	}
}

func TestPlanFiltersChecks(t *testing.T) {
	checks := []check.Check{
		{Name: "vet"},
		{Name: "vuln", WholeModule: true},
		{Name: "test", AllOnly: true},
		{Name: "off"},
	}
	targets := []project.Target{{Dir: "/m", Packages: []string{"./a"}}}
	jobs := Plan(checks, targets, false, func(n string) bool { return n == "off" })

	var names []string
	for _, j := range jobs {
		names = append(names, j.Check.Name)
	}

	if len(names) != 2 || names[0] != "vet" || names[1] != "vuln" {
		t.Fatalf("staged jobs = %v, want [vet vuln]", names)
	}

	if !jobs[1].Scope.All || jobs[0].Scope.All {
		t.Errorf("only the WholeModule Check should get an All scope")
	}

	if got := len(Plan(checks, targets, true, nil)); got != 4 {
		t.Errorf("--all jobs = %d, want 4", got)
	}
}

func TestExitCode(t *testing.T) {
	r := func(s check.Status) check.Result { return check.Result{Status: s} }

	tests := []struct {
		results []check.Result
		want    int
	}{
		{[]check.Result{r(check.StatusPassed)}, 0},
		{[]check.Result{r(check.StatusPassed), r(check.StatusFindings)}, 1},
		{[]check.Result{r(check.StatusFindings), r(check.StatusError)}, 2},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.results); got != tt.want {
			t.Errorf("ExitCode = %d, want %d", got, tt.want)
		}
	}
}
