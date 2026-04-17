package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeDirectoryPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		resolved string
		expect   string
	}{
		{
			name:     "tilde path keeps tilde",
			input:    "~/workspace/project",
			resolved: "/tmp/unused",
			expect:   "~/workspace/project/**",
		},
		{
			name:     "absolute path adds wildcard suffix",
			input:    "/tmp/project",
			resolved: "/tmp/project",
			expect:   "/tmp/project/**",
		},
		{
			name:     "existing wildcard suffix unchanged",
			input:    "~/workspace/project/**",
			resolved: "/tmp/unused",
			expect:   "~/workspace/project/**",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDirectoryPattern(tt.input, tt.resolved)
			if got != tt.expect {
				t.Fatalf("expected %q, got %q", tt.expect, got)
			}
		})
	}
}

func TestNormalizeDirectoryPatternRewritesExpandedHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	resolved := filepath.Join(home, "workspace", "demo")
	got := normalizeDirectoryPattern(resolved, resolved)
	expect := "~/workspace/demo/**"
	if got != expect {
		t.Fatalf("expected %q, got %q", expect, got)
	}
}

func TestRunAddDirUpdatesBothPermissionMaps(t *testing.T) {
	home := t.TempDir()
	projectDir := filepath.Join(home, "workspace", "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}

	initialConfig := `{"permission":{"read":{},"external_directory":{}}}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0o644); err != nil {
		t.Fatalf("writing initial config: %v", err)
	}

	t.Setenv("HOME", home)
	if err := runAddDir("~/workspace/demo", defaultOpencodeConfigPath); err != nil {
		t.Fatalf("runAddDir returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshaling config: %v", err)
	}

	permission := config["permission"].(map[string]any)
	read := permission["read"].(map[string]any)
	external := permission["external_directory"].(map[string]any)

	key := "~/workspace/demo/**"
	if read[key] != "allow" {
		t.Fatalf("expected read permission for %q", key)
	}
	if external[key] != "allow" {
		t.Fatalf("expected external_directory permission for %q", key)
	}
}

func TestRunAddDirRejectsMissingDirectory(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}

	initialConfig := `{"permission":{"read":{},"external_directory":{}}}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0o644); err != nil {
		t.Fatalf("writing initial config: %v", err)
	}

	t.Setenv("HOME", home)
	err := runAddDir("~/workspace/missing", defaultOpencodeConfigPath)
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}
