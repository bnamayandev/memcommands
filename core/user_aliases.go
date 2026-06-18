// User-defined aliases let the user attach a searchable label to a specific
// command from inside the TUI. They are persisted separately from the shell's
// own aliases so they survive across runs.
package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// userAliasPath returns the on-disk location for user-defined aliases,
// honouring XDG_CONFIG_HOME and falling back to ~/.config.
func userAliasPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", errors.New("HOME environment variable is not set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "memcommands", "aliases.json"), nil
}

// LoadUserAliases reads the saved alias→command map. A missing file is not an
// error: it simply means the user hasn't created any aliases yet.
func LoadUserAliases() map[string]string {
	path, err := userAliasPath()
	if err != nil {
		return map[string]string{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}

	aliases := make(map[string]string)
	if err := json.Unmarshal(data, &aliases); err != nil {
		return map[string]string{}
	}
	return aliases
}

// SaveUserAliases writes the alias→command map back to disk, creating the
// config directory as needed.
func SaveUserAliases(aliases map[string]string) error {
	path, err := userAliasPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(aliases, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// BuildUserAliasIndex turns an alias→command map into a normalized
// command→[]alias index for fuzzy matching.
func BuildUserAliasIndex(aliases map[string]string) map[string][]string {
	byCommand := make(map[string][]string, len(aliases))
	for alias, command := range aliases {
		key := normalizeCommandKey(command)
		if key == "" || alias == "" {
			continue
		}
		if !containsString(byCommand[key], alias) {
			byCommand[key] = append(byCommand[key], alias)
		}
	}
	return byCommand
}
