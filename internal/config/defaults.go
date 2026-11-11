package config

// DefaultGolangCI is written by `init` when the project has no golangci-lint config.
const DefaultGolangCI = `version: "2"
linters:
  default: standard
  enable:
    - bodyclose
    - errcheck
    - errorlint
    - gocritic
    - gosec
    - govet
    - ineffassign
    - misspell
    - staticcheck
    - unconvert
    - unused
    - wsl_v5
  exclusions:
    rules:
      - path: _test\.go
        linters:
          - gosec
  settings:
    errcheck:
      exclude-functions:
        - fmt.Fprint
        - fmt.Fprintf
        - fmt.Fprintln
`

// DefaultSuite is written by `init` when the project has no .gochecks.yml.
const DefaultSuite = `# jobs: 4          # max Checks running at once (default: GOMAXPROCS)
# timeout: 5m       # per-Check timeout
# disable: []       # Check names to skip: gofmt, vet, golangci-lint, govulncheck, gosec, test
`
