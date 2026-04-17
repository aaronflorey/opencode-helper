package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aaronflorey/opencode-helper/internal/store"

	"github.com/spf13/cobra"
)

const defaultOpencodeConfigPath = "~/.config/opencode/opencode.json"

func NewAddDirCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-dir <directory>",
		Short: "Add an external directory allow rule to OpenCode config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddDir(args[0], defaultOpencodeConfigPath)
		},
	}

	return cmd
}

func runAddDir(rawDir string, rawConfigPath string) error {
	resolvedDirPath, err := store.ExpandPath(rawDir)
	if err != nil {
		return fmt.Errorf("expanding directory path: %w", err)
	}

	info, err := os.Stat(resolvedDirPath)
	if err != nil {
		return fmt.Errorf("checking directory %s: %w", resolvedDirPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", resolvedDirPath)
	}

	resolvedConfigPath, err := store.ExpandPath(rawConfigPath)
	if err != nil {
		return fmt.Errorf("expanding config path: %w", err)
	}

	config, err := loadOpencodeConfig(resolvedConfigPath)
	if err != nil {
		return err
	}

	pattern := normalizeDirectoryPattern(rawDir, resolvedDirPath)
	updated, err := ensureDirectoryPermissions(config, pattern)
	if err != nil {
		return err
	}

	if !updated {
		fmt.Fprintf(os.Stderr, "Directory already allowed: %s\n", pattern)
		return nil
	}

	if err := saveOpencodeConfig(resolvedConfigPath, config); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Added directory permissions: %s\n", pattern)
	return nil
}

func loadOpencodeConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading OpenCode config %s: %w", path, err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing OpenCode config %s: %w", path, err)
	}

	return config, nil
}

func saveOpencodeConfig(path string, config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("serializing OpenCode config: %w", err)
	}

	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing OpenCode config %s: %w", path, err)
	}

	return nil
}

func ensureDirectoryPermissions(config map[string]any, pattern string) (bool, error) {
	permissionMap, err := getOrCreateMap(config, "permission")
	if err != nil {
		return false, err
	}

	readMap, err := getOrCreateMap(permissionMap, "read")
	if err != nil {
		return false, err
	}

	externalDirectoryMap, err := getOrCreateMap(permissionMap, "external_directory")
	if err != nil {
		return false, err
	}

	updated := false
	if current, ok := readMap[pattern].(string); !ok || current != "allow" {
		readMap[pattern] = "allow"
		updated = true
	}

	if current, ok := externalDirectoryMap[pattern].(string); !ok || current != "allow" {
		externalDirectoryMap[pattern] = "allow"
		updated = true
	}

	return updated, nil
}

func getOrCreateMap(parent map[string]any, key string) (map[string]any, error) {
	value, ok := parent[key]
	if !ok {
		child := map[string]any{}
		parent[key] = child
		return child, nil
	}

	child, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("config field %q must be an object", key)
	}

	return child, nil
}

func normalizeDirectoryPattern(rawDir string, resolvedDirPath string) string {
	trimmed := strings.TrimSpace(rawDir)

	var normalized string
	if strings.HasPrefix(trimmed, "~") {
		normalized = filepath.Clean(trimmed)
	} else {
		normalized = filepath.Clean(trimmed)
		if normalized == "." && resolvedDirPath != "" {
			normalized = filepath.Clean(resolvedDirPath)
		}
	}

	normalized = rewriteHomePathToTilde(normalized)

	if strings.HasSuffix(normalized, "/**") {
		return normalized
	}
	if normalized == string(filepath.Separator) {
		return normalized + "**"
	}

	return normalized + string(filepath.Separator) + "**"
}

func rewriteHomePathToTilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	home = filepath.Clean(home)
	path = filepath.Clean(path)
	if path == home {
		return "~"
	}

	prefix := home + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(filepath.Separator) + strings.TrimPrefix(path, prefix)
	}

	return path
}
