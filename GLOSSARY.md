# gochecks

An installable tool that orchestrates quality and security tooling for Go projects, installs a pre-commit hook, and validates itself.

## Language

**Check**:
One invocation of a wrapped tool against a project, producing a pass/fail result.
_Avoid_: Linter, scan, task, step

**Finding**:
A single issue reported by a Check.
_Avoid_: Warning, violation, error, vulnerability (unless it is a Finding of a vulnerability Check)
