package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("SHG_HOME", home)
	t.Setenv("SHG_SOCKET", filepath.Join(home, "custom.sock"))
	t.Setenv("SHG_DB", filepath.Join(home, "custom.db"))
	t.Setenv("SHG_OUTPUT_DIR", filepath.Join(home, "custom-outputs"))
	t.Setenv("SHG_SHELL", "/bin/bash")
	t.Setenv("SHG_INLINE_OUTPUT_LIMIT", "123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.HomeDir != home {
		t.Fatalf("unexpected home dir: %q", cfg.HomeDir)
	}
	if cfg.ShellPath != "/bin/bash" {
		t.Fatalf("unexpected shell path: %q", cfg.ShellPath)
	}
	if cfg.InlineOutputLimit != 123 {
		t.Fatalf("unexpected inline limit: %d", cfg.InlineOutputLimit)
	}
}

func TestLoadRejectsUnsupportedShell(t *testing.T) {
	t.Setenv("SHG_HOME", t.TempDir())
	t.Setenv("SHG_SHELL", "/bin/fish")
	if _, err := Load(); err == nil {
		t.Fatal("expected unsupported shell to fail")
	}
}
