// Package hook installs the git pre-commit hook.
package hook

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "# >>> gochecks >>>"
	endMarker   = "# <<< gochecks <<<"
)

// ErrExists is returned by Install when a foreign hook exists and neither
// chaining nor replacing was requested.
var ErrExists = errors.New("a pre-commit hook already exists")

// Mode says what to do when a hook file already exists.
type Mode int

const (
	// Abort refuses to touch a foreign hook.
	Abort Mode = iota
	// Chain appends our block to the existing hook.
	Chain
)

// Path returns the pre-commit hook path for the repository containing dir,
// honouring core.hooksPath and worktrees.
func Path(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-path", "hooks/pre-commit")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("locating git hooks (is this a git repository?): %w", err)
	}

	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}

	return p, nil
}

// Prefix returns dir's path relative to the repository root, slash-separated
// with no trailing slash ("" at the root).
func Prefix(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-prefix")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(strings.TrimSpace(string(out)), "/"), nil
}

// Block returns the hook snippet that runs the tool from the project directory.
func Block(prefix string) string {
	var b strings.Builder
	b.WriteString(beginMarker + "\n")

	if prefix != "" {
		fmt.Fprintf(&b, "cd %q || exit 1\n", prefix)
	}

	b.WriteString("if command -v gochecks >/dev/null 2>&1; then\n")
	b.WriteString("  gochecks run --no-tui || exit $?\n")
	b.WriteString("elif go tool -n gochecks >/dev/null 2>&1; then\n")
	b.WriteString("  go tool gochecks run --no-tui || exit $?\n")
	b.WriteString("else\n")
	b.WriteString("  echo \"gochecks not found on PATH or as a go tool\" >&2\n")
	b.WriteString("  exit 2\n")
	b.WriteString("fi\n")
	b.WriteString(endMarker + "\n")

	return b.String()
}

// Exists reports whether the hook file exists and whether it already
// contains our block.
func Exists(path string) (exists, ours bool, err error) {
	data, err := readHook(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}

	if err != nil {
		return false, false, err
	}

	return true, strings.Contains(string(data), beginMarker), nil
}

// Install writes or updates the hook. Our own block is replaced in place, so
// Install is idempotent.
func Install(path, prefix string, mode Mode) error {
	exists, ours, err := Exists(path)
	if err != nil {
		return err
	}

	block := Block(prefix)

	var content string

	switch {
	case !exists:
		content = "#!/bin/sh\n" + block
	case ours:
		data, err := readHook(path)
		if err != nil {
			return err
		}

		content = replaceBlock(string(data), block)
	case mode == Chain:
		data, err := readHook(path)
		if err != nil {
			return err
		}

		content = strings.TrimRight(string(data), "\n") + "\n\n" + block
	default:
		return ErrExists
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// Git only runs hooks that are executable.
	return os.WriteFile(path, []byte(content), 0o755) // #nosec G306 G703 -- path comes from `git rev-parse --git-path`
}

func readHook(path string) ([]byte, error) {
	return os.ReadFile(path) // #nosec G304 -- path comes from `git rev-parse --git-path`
}

func replaceBlock(content, block string) string {
	start := strings.Index(content, beginMarker)

	end := strings.Index(content, endMarker)
	if start < 0 || end < start {
		return content
	}

	end += len(endMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}

	return content[:start] + block + content[end:]
}
