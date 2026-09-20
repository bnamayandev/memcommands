// Pinned commands are user favorites that float to the top of the results list,
// stored as a plain list of command strings that survives restarts.
package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func pinnedPath(appName string) (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", errors.New("HOME environment variable is not set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, appName, "pinned.json"), nil
}

func LoadPinnedCommands(appName string) []string {
	path, err := pinnedPath(appName)
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var commands []string
	if err := json.Unmarshal(data, &commands); err != nil {
		return nil
	}
	return commands
}

func SavePinnedCommands(appName string, commands []string) error {
	path, err := pinnedPath(appName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(commands, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
