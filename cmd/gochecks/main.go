// Command gochecks orchestrates quality and security Checks for Go projects.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"

	"golang.org/x/term"

	"github.com/YouweGit/gochecks/internal/check"
	"github.com/YouweGit/gochecks/internal/config"
	"github.com/YouweGit/gochecks/internal/hook"
	"github.com/YouweGit/gochecks/internal/project"
	"github.com/YouweGit/gochecks/internal/report"
	"github.com/YouweGit/gochecks/internal/runner"
	"github.com/YouweGit/gochecks/internal/tools"
	"github.com/YouweGit/gochecks/internal/tui"
)

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

// buildVersion prefers the ldflags version, then the module version recorded
// by `go install` / `go tool`, and finally "dev".
func buildVersion() string {
	if version != "dev" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return version
}

const usage = `Usage: gochecks <command> [flags]

Commands:
  init         Write default configs and install the git pre-commit hook
  run          Run Checks on staged packages (--all for the whole module)
  doctor       Report which wrapped tools are installed
  version      Print the version
  completion   Print a shell completion script (bash, zsh, fish, powershell)
  help         Show help for a command: gochecks help <command>
`

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

func execute(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)

		return 0
	}

	if len(args) > 1 && isHelpFlag(args[1]) && args[0] != "help" {
		return printHelp(args[0], stdout, stderr)
	}

	var err error

	code := 0

	switch args[0] {
	case "run":
		code, err = cmdRun(args[1:], stdout, stderr)
	case "init":
		err = cmdInit(args[1:], stdout)
	case "doctor":
		code, err = cmdDoctor(stdout)
	case "version":
		fmt.Fprintln(stdout, buildVersion())
	case "completion":
		err = cmdCompletion(args[1:], stdout)
	case "-h", "--help", "-help":
		fmt.Fprint(stdout, usage)
	case "help":
		if len(args) > 1 {
			return printHelp(args[1], stdout, stderr)
		}

		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)

		return 2
	}

	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	return code
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func confirm(prompt string) bool {
	fmt.Print(prompt + " [y/N] ")

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))

	return ans == "y" || ans == "yes"
}

func cmdRun(args []string, stdout, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts runOptions

	opts.register(fs)

	if err := fs.Parse(args); err != nil {
		return 2, nil
	}

	all, noTUI, format, jobsN := &opts.all, &opts.noTUI, &opts.format, &opts.jobs

	if *format != "text" && *format != "json" {
		return 0, fmt.Errorf("unknown format %q", *format)
	}

	wd, err := os.Getwd()
	if err != nil {
		return 0, err
	}

	cfg, err := config.Load(wd)
	if err != nil {
		return 0, err
	}

	timeout, _ := cfg.TimeoutDuration()
	if *jobsN > 0 {
		cfg.Jobs = *jobsN
	}

	modules, err := project.Modules(wd)
	if err != nil {
		return 0, err
	}

	var (
		targets []project.Target
		changes changeSet
	)

	if *all {
		targets = project.All(modules)
	} else {
		changes.Staged, err = project.Staged(wd)
		if err != nil {
			return 0, err
		}

		targets = project.Plan(modules, changes.Staged)
	}

	jobs := runner.Plan(tools.Default(), targets, *all, cfg.Disabled)
	if len(jobs) == 0 {
		if *format == "json" {
			return 0, report.JSON(stdout, nil)
		}

		if *all {
			fmt.Fprintln(stdout, "Nothing to check: every Check is disabled in .gochecks.yml or no Go module was found.")

			return 0, nil
		}

		changes.AllDisabled = len(targets) > 0
		// Unstaged/untracked are best-effort hints; errors only drop the hint.
		changes.Unstaged, _ = project.Unstaged(wd)
		changes.Untracked, _ = project.Untracked(wd)

		fmt.Fprint(stdout, nothingToCheck(changes, wd))

		return 0, nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	interactive := isInteractive() && !*noTUI && *format == "text"
	if interactive {
		offerInstalls(jobs)
	}

	r := check.ExecRunner{Timeout: timeout}

	var results []check.Result

	if interactive {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		results, err = tui.Run(jobs, cancel, func(send func(runner.Update)) []check.Result {
			return runner.Run(ctx, r, jobs, cfg.Jobs, send)
		})
		if err != nil {
			return 0, err
		}
	} else {
		results = runner.Run(ctx, r, jobs, cfg.Jobs, nil)
	}

	if *format == "json" {
		if err := report.JSON(stdout, results); err != nil {
			return 0, err
		}
	} else {
		report.Plain(stdout, results)
	}

	return runner.ExitCode(results), nil
}

// offerInstalls prompts to install each missing wrapped tool.
func offerInstalls(jobs []runner.Job) {
	seen := map[string]bool{}

	for _, j := range jobs {
		c := j.Check
		if seen[c.Binary] || c.Install == "" {
			continue
		}

		seen[c.Binary] = true
		if _, err := exec.LookPath(c.Binary); err == nil {
			continue
		}

		if !confirm(fmt.Sprintf("%s is not installed. Run `%s`?", c.Binary, c.Install)) {
			continue
		}

		parts := strings.Fields(c.Install)
		cmd := exec.Command(parts[0], parts[1:]...) // #nosec G204 -- Install commands are built-in constants

		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
		}
	}
}

func cmdInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if _, err := project.Modules(wd); err != nil {
		return err
	}

	interactive := isInteractive()

	files := []struct{ name, content string }{
		{".golangci.yml", config.DefaultGolangCI},
		{config.FileName, config.DefaultSuite},
	}
	for _, f := range files {
		path := filepath.Join(wd, f.name)
		if _, err := os.Stat(path); err == nil {
			if !interactive || !confirm(f.name+" exists. Overwrite?") {
				fmt.Fprintf(stdout, "kept existing %s\n", f.name)
				continue
			}
		}

		if err := os.WriteFile(path, []byte(f.content), 0o600); err != nil {
			return err
		}

		fmt.Fprintf(stdout, "wrote %s\n", f.name)
	}

	hookPath, err := hook.Path(wd)
	if err != nil {
		return err
	}

	prefix, err := hook.Prefix(wd)
	if err != nil {
		return err
	}

	mode := hook.Abort

	if exists, ours, err := hook.Exists(hookPath); err != nil {
		return err
	} else if exists && !ours {
		if !interactive {
			return fmt.Errorf("%s already exists; run `init` in a terminal to chain it", hookPath)
		}

		if !confirm("A pre-commit hook already exists. Chain gochecks after it?") {
			return errors.New("aborted: existing pre-commit hook left untouched")
		}

		mode = hook.Chain
	}

	if err := hook.Install(hookPath, prefix, mode); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "installed pre-commit hook at %s\n", hookPath)

	return nil
}

func cmdDoctor(stdout io.Writer) (int, error) {
	missing := 0
	report := func(name, install string, err error) {
		if err != nil {
			missing++

			hint := ""
			if install != "" {
				hint = "  install: " + install
			}

			fmt.Fprintf(stdout, "MISSING %s%s\n", name, hint)

			return
		}

		fmt.Fprintf(stdout, "ok      %s\n", name)
	}

	seen := map[string]bool{}
	for _, c := range tools.Default() {
		if seen[c.Binary] {
			continue
		}

		seen[c.Binary] = true
		_, err := exec.LookPath(c.Binary)
		report(c.Binary, c.Install, err)
	}

	for _, b := range []string{"git", "sh"} {
		_, err := exec.LookPath(b)
		report(b, "", err)
	}

	if missing > 0 {
		return 1, nil
	}

	return 0, nil
}
