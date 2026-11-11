package project

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlan(t *testing.T) {
	root := filepath.FromSlash("/repo")
	sub := filepath.Join(root, "tools")
	abs := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }

	tests := []struct {
		name    string
		modules []string
		staged  []string
		want    []Target
	}{
		{
			name:    "go files map to packages",
			modules: []string{root},
			staged:  []string{abs("a.go"), abs("internal", "x", "x.go"), abs("internal", "x", "y.go"), abs("README.md")},
			want:    []Target{{Dir: root, Packages: []string{".", "./internal/x"}}},
		},
		{
			name:    "go.mod makes the module wide",
			modules: []string{root},
			staged:  []string{abs("go.mod")},
			want:    []Target{{Dir: root, ModuleWide: true}},
		},
		{
			name:    "nested module owns its files",
			modules: []string{root, sub},
			staged:  []string{filepath.Join(sub, "cmd", "m.go")},
			want:    []Target{{Dir: sub, Packages: []string{"./cmd"}}},
		},
		{
			name:    "non-go files only",
			modules: []string{root},
			staged:  []string{abs("README.md")},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Plan(tt.modules, tt.staged); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Plan() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
