package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func GetHistoryLines() ([]string, error) {
	shellPath := os.Getenv("SHELL")
	home := os.Getenv("HOME")

	if home == "" {
		return nil, errors.New("HOME environment variable is not set")
	}

	shellName := filepath.Base(shellPath)

	var filePath string
	switch shellName {
	case "bash":
		filePath = filepath.Join(home, ".bash_history")
	case "zsh":
		filePath = filepath.Join(home, ".zsh_history")
	case "fish":
		filePath = filepath.Join(home, ".local", "share", "fish", "fish_history")
	default:
		return nil, fmt.Errorf("unsupported shell: %q", shellName)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	historyList := make([]string, 0, 1024)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		historyList = append(historyList, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed while reading file %s: %w", filePath, err)
	}

	return historyList, nil
}
