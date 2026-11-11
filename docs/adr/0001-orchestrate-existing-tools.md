# Orchestrate existing tools instead of reimplementing checks

gochecks runs established tools (`go vet`, `golangci-lint`, `govulncheck`, `gosec`, `go test`) as subprocesses found via `PATH` and aggregates their results; it never implements checks itself. This keeps the tool small and lets projects get upstream improvements without upgrading it.

## Considered Options

- **Embed analyzers as Go libraries** (`golang.org/x/tools/go/analysis`): a single self-contained binary, but we'd own version pinning and lag upstream, and `golangci-lint` and `govulncheck` aren't practical to embed.
- **Bundle binaries in releases**: no install prompts, but large artifacts and per-OS packaging for every tool.

## Consequences

- Wrapped tools must be installed; `doctor` and the interactive install prompt handle this.
- Output formats of each tool are a compatibility surface we must parse (prefer their JSON output).
