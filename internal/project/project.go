// Package project locates Go modules and maps staged files onto them.
package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoModule is returned when no go.mod or go.work is found.
var ErrNoModule = errors.New("no go.mod or go.work found")

// Target is one module and what to analyse in it.
type Target struct {
	Dir      string
	Packages []string
	// ModuleWide is set when go.mod or go.sum changed.
	ModuleWide bool
}

func findUp(dir, name string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
}

// Modules returns the module directories that apply to dir: every `use`d
// module of the enclosing go.work, or the nearest module otherwise.
func Modules(dir string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	if wd, ok := findUp(dir, "go.work"); ok {
		cmd := exec.Command("go", "work", "edit", "-json")
		cmd.Dir = wd

		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("reading go.work: %w", err)
		}

		var w struct{ Use []struct{ DiskPath string } }
		if err := json.Unmarshal(out, &w); err != nil {
			return nil, fmt.Errorf("parsing go.work: %w", err)
		}

		var mods []string

		for _, u := range w.Use {
			p := u.DiskPath
			if !filepath.IsAbs(p) {
				p = filepath.Join(wd, p)
			}

			mods = append(mods, filepath.Clean(p))
		}

		if len(mods) > 0 {
			return mods, nil
		}
	}

	if root, ok := findUp(dir, "go.mod"); ok {
		return []string{root}, nil
	}

	return nil, ErrNoModule
}

// All returns whole-module targets.
func All(modules []string) []Target {
	ts := make([]Target, 0, len(modules))
	for _, m := range modules {
		ts = append(ts, Target{Dir: m, ModuleWide: true})
	}

	return ts
}

// Plan maps absolute staged file paths onto modules. A file belongs to the
// deepest module containing it; modules without relevant files are omitted.
func Plan(modules, staged []string) []Target {
	byDir := map[string]*Target{}
	pkgs := map[string]map[string]bool{}

	for _, f := range staged {
		mod := ""
		for _, m := range modules {
			if within(m, f) && len(m) > len(mod) {
				mod = m
			}
		}

		if mod == "" {
			continue
		}

		t := byDir[mod]
		if t == nil {
			t = &Target{Dir: mod}
			byDir[mod] = t
			pkgs[mod] = map[string]bool{}
		}

		switch base := filepath.Base(f); {
		case base == "go.mod" || base == "go.sum":
			if filepath.Dir(f) == mod {
				t.ModuleWide = true
			}
		case strings.HasSuffix(base, ".go"):
			rel, err := filepath.Rel(mod, filepath.Dir(f))
			if err != nil {
				continue
			}

			rel = filepath.ToSlash(rel)
			if rel == "." {
				pkgs[mod]["."] = true
			} else {
				pkgs[mod]["./"+rel] = true
			}
		}
	}

	var out []Target

	for dir, t := range byDir {
		for p := range pkgs[dir] {
			t.Packages = append(t.Packages, p)
		}

		sort.Strings(t.Packages)

		if t.ModuleWide || len(t.Packages) > 0 {
			out = append(out, *t)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })

	return out
}

func within(dir, file string) bool {
	rel, err := filepath.Rel(dir, file)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Staged returns absolute paths of added, copied, modified or renamed staged files.
func Staged(dir string) ([]string, error) {
	return gitFiles(dir, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
}

// Unstaged returns absolute paths of tracked files with unstaged changes.
func Unstaged(dir string) ([]string, error) {
	return gitFiles(dir, "diff", "--name-only", "-z")
}

// Untracked returns absolute paths of untracked, non-ignored files.
func Untracked(dir string) ([]string, error) {
	return gitFiles(dir, "ls-files", "--others", "--exclude-standard", "--full-name", "-z")
}

// Relevant reports whether a changed file can affect any Check.
func Relevant(path string) bool {
	base := filepath.Base(path)

	return strings.HasSuffix(base, ".go") || base == "go.mod" || base == "go.sum"
}

func gitFiles(dir string, args ...string) ([]string, error) {
	top, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}

	root := filepath.Clean(strings.TrimSpace(string(top)))

	out, err := gitOutput(dir, args...)
	if err != nil {
		return nil, err
	}

	var files []string

	for _, f := range bytes.Split(out, []byte{0}) {
		if len(f) > 0 {
			files = append(files, filepath.Join(root, filepath.FromSlash(string(f))))
		}
	}

	return files, nil
}

func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) // #nosec G204 -- args are fixed by callers in this package
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return out, nil
}
