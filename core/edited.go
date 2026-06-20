// Edited commands rewrite a history command in place from the TUI, stored as a
// normalized-original-key → edited-text map so edits survive navigation/restarts.
package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func editedPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", errors.New("HOME environment variable is not set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "memcommands", "edited.json"), nil
}

func LoadEditedCommands() map[string]string {
	path, err := editedPath()
	if err != nil {
		return map[string]string{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}

	edits := make(map[string]string)
	if err := json.Unmarshal(data, &edits); err != nil {
		return map[string]string{}
	}
	return edits
}

func SaveEditedCommands(edits map[string]string) error {
	path, err := editedPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(edits, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
