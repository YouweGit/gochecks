# gochecks

Runs Go quality and security Checks (`gofmt`, `go vet`, `golangci-lint`, `govulncheck`, `gosec`, `go test -race`) in parallel, with a live TUI, and installs a git pre-commit hook.

## Install

```sh
go install github.com/YouweGit/gochecks/cmd/gochecks@latest
```

Or pin it per project as a Go tool (Go 1.24+), which records the version in `go.mod`:

```sh
go get -tool github.com/YouweGit/gochecks/cmd/gochecks@latest
go tool gochecks init
```

The pre-commit hook uses the binary on `PATH` if present, otherwise `go tool gochecks`.

### Try a local checkout

To test local changes in another project, point its `go.mod` at your checkout:

```sh
cd /path/to/other-project
go mod edit \
  -require=github.com/YouweGit/gochecks@v0.0.0 \
  -replace=github.com/YouweGit/gochecks=/path/to/gochecks \
  -tool=github.com/YouweGit/gochecks/cmd/gochecks
go mod tidy
go tool gochecks run --all
```

Revert with `go mod edit -dropreplace=github.com/YouweGit/gochecks -droptool=github.com/YouweGit/gochecks/cmd/gochecks`, then `go mod tidy`.

Or download a release binary (Linux, macOS, Windows). The wrapped tools must be on `PATH`; `gochecks doctor` shows what is missing and `run` offers to install them in a terminal.

## Usage

```sh
gochecks init      # write .golangci.yml / .gochecks.yml and install the pre-commit hook
gochecks run       # Checks on staged packages (what the hook runs)
gochecks run --all # whole module, including tests
gochecks doctor    # report missing tools
```

`run` flags: `--all`, `--no-tui`, `--format text|json`, `--jobs N`. `gochecks help <command>` shows details.

### Shell completion

```sh
source <(gochecks completion bash)                       # bash
gochecks completion zsh > "${fpath[1]}/_gochecks"        # zsh
gochecks completion fish > ~/.config/fish/completions/gochecks.fish
```

PowerShell is supported too. Completion needs the installed binary; `go tool gochecks` can't be tab-completed because Go owns that completion.

Exit codes: `0` pass, `1` Findings, `2` tool or usage error (missing tool, timeout, bad config).

`init` never overwrites existing configs silently and asks before chaining an existing pre-commit hook. Bypass the hook with `git commit --no-verify`.

## Configuration

Optional `.gochecks.yml`:

```yaml
jobs: 4          # max Checks running at once (default: GOMAXPROCS)
timeout: 5m      # per-Check timeout; a timeout is a tool error (exit 2)
disable: [gosec] # Check names to skip
```

Requires golangci-lint v2. Terms are defined in [GLOSSARY.md](GLOSSARY.md); design decisions in [docs/adr](docs/adr).
