package main

import (
	"flag"
	"fmt"
	"io"
)

type runOptions struct {
	all    bool
	noTUI  bool
	format string
	jobs   int
}

func (o *runOptions) register(fs *flag.FlagSet) {
	fs.BoolVar(&o.all, "all", false, "analyse the whole module and include slow Checks (tests)")
	fs.BoolVar(&o.noTUI, "no-tui", false, "plain output even on a terminal")
	fs.StringVar(&o.format, "format", "text", "output format: text or json")
	fs.IntVar(&o.jobs, "jobs", 0, "max Checks running at once (default: GOMAXPROCS)")
}

var commandHelp = map[string]string{
	"init": `Usage: gochecks init

Writes .golangci.yml and .gochecks.yml (never overwriting without asking)
and installs the git pre-commit hook. Asks before chaining an existing hook.
`,
	"run": `Usage: gochecks run [flags]

Runs Checks on the packages touched by staged files. With --all, runs on the
whole module and includes the test Check.

Exit codes: 0 pass, 1 Findings, 2 tool or usage error.

Flags:
`,
	"doctor": `Usage: gochecks doctor

Reports which wrapped tools (and git/sh) are installed, with install commands
for the missing ones. Exits 1 if anything is missing.
`,
	"version": `Usage: gochecks version

Prints the version.
`,
	"completion": `Usage: gochecks completion <bash|zsh|fish|powershell>

Prints a shell completion script for the installed binary. Completion does not
work through "go tool gochecks"; install the binary for that.

  bash:        source <(gochecks completion bash)
  zsh:         gochecks completion zsh > "${fpath[1]}/_gochecks"
  fish:        gochecks completion fish > ~/.config/fish/completions/gochecks.fish
  powershell:  gochecks completion powershell | Out-String | Invoke-Expression
`,
}

func isHelpFlag(s string) bool {
	return s == "-h" || s == "--help" || s == "-help"
}

// printHelp writes help for one command and returns the exit code.
func printHelp(name string, stdout, stderr io.Writer) int {
	text, ok := commandHelp[name]
	if !ok {
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", name, usage)

		return 2
	}

	fmt.Fprint(stdout, text)

	if name == "run" {
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fs.SetOutput(stdout)
		(&runOptions{}).register(fs)
		fs.PrintDefaults()
	}

	return 0
}
