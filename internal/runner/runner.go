// Package runner schedules Checks across modules with bounded concurrency.
package runner

import (
	"context"
	"sync"

	"github.com/YouweGit/gochecks/internal/check"
	"github.com/YouweGit/gochecks/internal/project"
)

// Job is one Check applied to one module.
type Job struct {
	Check check.Check
	Scope check.Scope
}

// Plan expands checks and targets into jobs, dropping Checks that don't apply.
func Plan(checks []check.Check, targets []project.Target, all bool, skip func(name string) bool) []Job {
	var jobs []Job

	for _, t := range targets {
		for _, c := range checks {
			if (skip != nil && skip(c.Name)) || (c.AllOnly && !all) {
				continue
			}

			s := check.Scope{Dir: t.Dir, Packages: t.Packages}
			switch {
			case c.WholeModule || all:
				s.All = true
			case len(t.Packages) == 0:
				continue
			}

			jobs = append(jobs, Job{Check: c, Scope: s})
		}
	}

	return jobs
}

// Update reports a job changing state; Result is set once Status is final.
type Update struct {
	Index  int
	Status check.Status
	Result check.Result
}

// Run executes jobs with at most jobsN running concurrently. Jobs whose Check
// mutates files run one at a time afterwards. Results are index-aligned with
// jobs. onUpdate may be nil and is called from multiple goroutines.
func Run(ctx context.Context, r check.Runner, jobs []Job, jobsN int, onUpdate func(Update)) []check.Result {
	if jobsN < 1 {
		jobsN = 1
	}

	if onUpdate == nil {
		onUpdate = func(Update) {}
	}

	results := make([]check.Result, len(jobs))

	exec := func(i int) {
		onUpdate(Update{Index: i, Status: check.StatusRunning})
		res := r.Run(ctx, jobs[i].Check, jobs[i].Scope)
		results[i] = res
		onUpdate(Update{Index: i, Status: res.Status, Result: res})
	}

	var wg sync.WaitGroup

	sem := make(chan struct{}, jobsN)

	var serial []int

	for i, j := range jobs {
		if j.Check.Mutates {
			serial = append(serial, i)
			continue
		}

		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			exec(i)
		}()
	}

	wg.Wait()

	for _, i := range serial {
		exec(i)
	}

	return results
}

// ExitCode maps results to the process exit code: 2 for tool errors,
// 1 for Findings, 0 otherwise.
func ExitCode(results []check.Result) int {
	code := 0

	for _, r := range results {
		switch r.Status {
		case check.StatusError:
			return 2
		case check.StatusFindings:
			code = 1
		}
	}

	return code
}
