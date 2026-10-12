package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSupportedConfigShapes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autorestic.yml")
	data := `version: 2
locations:
  home:
    from: [~/Documents, ~/Pictures]
    to:
      - archive
    options:
      forget:
        keep-daily: 7
  projects:
    from: ~/src
    to: archive
backends:
  archive:
    type: local
    path: /backups/restic
    key: example-secret
    env:
      AWS_ACCESS_KEY_ID: example-access
      AWS_SECRET_ACCESS_KEY: example-secret-value
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cfg.Locations["home"].From); got != 2 {
		t.Fatalf("home sources = %d, want 2", got)
	}
	if got := cfg.Locations["projects"].From[0]; got != "~/src" {
		t.Fatalf("scalar source = %q, want ~/src", got)
	}
	if got := cfg.Backends["archive"].Env["AWS_SECRET_ACCESS_KEY"]; got != "example-secret-value" {
		t.Fatalf("backend env value = %q", got)
	}
}

func TestLoadExampleConfiguration(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "testdata", "autorestic.example.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Locations) != 2 || len(cfg.Backends) != 1 {
		t.Fatalf("example config contains %d locations and %d backends", len(cfg.Locations), len(cfg.Backends))
	}
}

func TestLoadErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "missing.yml"))
		if err == nil || !strings.Contains(err.Error(), "read Autorestic config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("invalid yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yml")
		_ = os.WriteFile(path, []byte("locations: [\n"), 0600)
		_, err := Load(path)
		if err == nil || !strings.Contains(err.Error(), "parse Autorestic config") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("missing backend reference", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yml")
		_ = os.WriteFile(path, []byte("locations:\n  home:\n    from: /home/user\n    to: nowhere\n"), 0600)
		_, err := Load(path)
		if err == nil || !strings.Contains(err.Error(), `undefined backend "nowhere"`) {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("missing backend path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yml")
		_ = os.WriteFile(path, []byte("backends:\n  archive:\n    type: local\n"), 0600)
		_, err := Load(path)
		if err == nil || !strings.Contains(err.Error(), "missing required field path") {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("unsupported version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yml")
		_ = os.WriteFile(path, []byte("version: 1\nbackends:\n  archive:\n    path: /repo\n"), 0600)
		_, err := Load(path)
		if err == nil || !strings.Contains(err.Error(), "version 2 is required") {
			t.Fatalf("Load() error = %v", err)
		}
	})
}
