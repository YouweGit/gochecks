package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks", "pre-commit")

	if err := Install(path, "svc", Abort); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "#!/bin/sh\n") || !strings.Contains(string(data), `cd "svc"`) {
		t.Errorf("unexpected hook:\n%s", data)
	}

	// Re-installing replaces our block instead of duplicating it.
	if err := Install(path, "", Abort); err != nil {
		t.Fatal(err)
	}

	data, _ = os.ReadFile(path)
	if strings.Count(string(data), beginMarker) != 1 || strings.Contains(string(data), "cd ") {
		t.Errorf("block not replaced in place:\n%s", data)
	}
}

func TestInstallForeignHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre-commit")

	foreign := "#!/bin/sh\necho mine\n"
	if err := os.WriteFile(path, []byte(foreign), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "", Abort); !errors.Is(err, ErrExists) {
		t.Fatalf("Install(Abort) = %v, want ErrExists", err)
	}

	if data, _ := os.ReadFile(path); string(data) != foreign {
		t.Fatal("foreign hook was modified")
	}

	if err := Install(path, "", Chain); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), foreign) || !strings.Contains(string(data), beginMarker) {
		t.Errorf("chain lost the original or missed our block:\n%s", data)
	}
}
