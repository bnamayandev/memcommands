package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func deletedPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", errors.New("HOME environment variable is not set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "memcommands", "deleted.json"), nil
}

func LoadDeletedCommands() []string {
	path, err := deletedPath()
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

func SaveDeletedCommands(commands []string) error {
	path, err := deletedPath()
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
