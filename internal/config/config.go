// Package config loads the optional .gochecks.yml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// FileName is the project-level config file name.
const FileName = ".gochecks.yml"

// Config holds user settings; zero values mean "use the default".
type Config struct {
	// Jobs is the maximum number of Checks running at once.
	Jobs int `yaml:"jobs"`
	// Timeout is the per-Check timeout, e.g. "5m".
	Timeout string `yaml:"timeout"`
	// Disable lists Check names to skip.
	Disable []string `yaml:"disable"`
}

// Load reads FileName from dir; a missing file yields the defaults.
func Load(dir string) (Config, error) {
	var c Config

	data, err := os.ReadFile(filepath.Join(dir, FileName)) // #nosec G304 -- fixed file name inside the project directory
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, err
	}

	if err == nil {
		if err := yaml.Unmarshal(data, &c); err != nil {
			return c, fmt.Errorf("parsing %s: %w", FileName, err)
		}
	}

	if c.Jobs <= 0 {
		c.Jobs = runtime.GOMAXPROCS(0)
	}

	if _, err := c.TimeoutDuration(); err != nil {
		return c, err
	}

	return c, nil
}

// TimeoutDuration returns the per-Check timeout, defaulting to five minutes.
func (c Config) TimeoutDuration() (time.Duration, error) {
	if c.Timeout == "" {
		return 5 * time.Minute, nil
	}

	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid timeout %q in %s", c.Timeout, FileName)
	}

	return d, nil
}

// Disabled reports whether a Check name is switched off.
func (c Config) Disabled(name string) bool {
	for _, d := range c.Disable {
		if d == name {
			return true
		}
	}

	return false
}
