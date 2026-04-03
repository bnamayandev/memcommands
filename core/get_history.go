package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func GetHistoryLines() ([]string, error) {
	shellPath := os.Getenv("SHELL")
	home := os.Getenv("HOME")

	if home == "" {
		return nil, errors.New("HOME environment variable is not set")
	}

	historyFiles, err := historyCandidates(home, filepath.Base(shellPath), os.Getenv("HISTFILE"))
	if err != nil {
		return nil, err
	}

	historyList := make([]string, 0, 1024)
	seen := make(map[string]struct{}, 1024)

	for _, historyFile := range historyFiles {
		lines, err := readHistoryFile(historyFile.path, historyFile.format)
		if err != nil {
			return nil, err
		}

		for _, line := range lines {
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			historyList = append(historyList, line)
		}
	}

	if len(historyList) == 0 {
		return nil, fmt.Errorf("no readable history entries found for shell %q", filepath.Base(shellPath))
	}

	return historyList, nil
}

type historyFile struct {
	path   string
	format string
}

func historyCandidates(home, shellName, histFile string) ([]historyFile, error) {
	files := []historyFile{
		{path: filepath.Join(home, ".bash_history"), format: "bash"},
		{path: filepath.Join(home, ".zsh_history"), format: "zsh"},
		{path: filepath.Join(home, ".zhistory"), format: "zsh"},
		{path: filepath.Join(home, ".local", "share", "fish", "fish_history"), format: "fish"},
	}

	if histFile != "" {
		files = append(files, historyFile{
			path:   expandPath(histFile, home),
			format: historyFormat(shellName, histFile),
		})
	}

	switch shellName {
	case "bash", "zsh", "fish", "":
	default:
		return nil, fmt.Errorf("unsupported shell: %q", shellName)
	}

	slices.SortStableFunc(files, func(a, b historyFile) int {
		return cmpHistoryFilePriority(a, b, shellName)
	})

	unique := make([]historyFile, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if file.path == "" {
			continue
		}
		if _, ok := seen[file.path]; ok {
			continue
		}
		seen[file.path] = struct{}{}
		unique = append(unique, file)
	}

	return unique, nil
}

func cmpHistoryFilePriority(a, b historyFile, shellName string) int {
	return historyFilePriority(a, shellName) - historyFilePriority(b, shellName)
}

func historyFilePriority(file historyFile, shellName string) int {
	if file.format == shellName {
		return 1
	}
	if strings.Contains(file.path, shellName+"_history") || strings.HasSuffix(file.path, "."+shellName+"_history") {
		return 2
	}
	return 3
}

func expandPath(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

func historyFormat(shellName, filePath string) string {
	base := filepath.Base(filePath)

	switch {
	case strings.Contains(base, "zsh"):
		return "zsh"
	case strings.Contains(base, "fish"):
		return "fish"
	case strings.Contains(base, "bash"):
		return "bash"
	case shellName != "":
		return shellName
	default:
		return "bash"
	}
}

func readHistoryFile(filePath, format string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make([]string, 0, 512)
	for scanner.Scan() {
		line := normalizeHistoryLine(scanner.Text(), format)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed while reading file %s: %w", filePath, err)
	}

	return lines, nil
}

func normalizeHistoryLine(line, format string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	switch format {
	case "zsh":
		if strings.HasPrefix(line, ": ") {
			if idx := strings.IndexByte(line, ';'); idx >= 0 && idx+1 < len(line) {
				line = line[idx+1:]
			}
		}
	case "fish":
		const prefix = "- cmd: "
		if !strings.HasPrefix(line, prefix) {
			return ""
		}
		line = strings.TrimPrefix(line, prefix)
		line = strings.ReplaceAll(line, `\n`, " ")
		line = strings.ReplaceAll(line, `\\`, `\`)
	case "bash":
		if strings.HasPrefix(line, "#") {
			if _, err := strconv.ParseInt(strings.TrimPrefix(line, "#"), 10, 64); err == nil {
				return ""
			}
		}
	}

	line = strings.Join(strings.Fields(line), " ")
	return strings.TrimSpace(line)
}
