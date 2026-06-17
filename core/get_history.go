package core

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func GetHistoryLines() ([]string, error) {
	shellPath := os.Getenv("SHELL")
	shellName := filepath.Base(shellPath)
	home := os.Getenv("HOME")

	if home == "" {
		return nil, errors.New("HOME environment variable is not set")
	}

	historyFiles, err := historyCandidates(home, shellName, os.Getenv("HISTFILE"))
	if err != nil {
		return nil, err
	}

	historyList, err := readFirstAvailableHistory(historyFiles)
	if err != nil {
		return nil, err
	}

	if len(historyList) == 0 {
		historyList, err = readInteractiveShellHistory(shellPath)
		if err != nil {
			return nil, err
		}
	}

	historyList = dedupeKeepLast(historyList)

	if len(historyList) == 0 {
		return nil, fmt.Errorf("no readable history entries found for shell %q", shellName)
	}

	return historyList, nil
}

func readInteractiveShellHistory(shellPath string) ([]string, error) {
	shellName := filepath.Base(shellPath)
	if shellPath == "" {
		return nil, nil
	}

	command := shellHistoryCommand(shellName)
	if command == "" {
		return nil, nil
	}

	cmd := exec.Command(shellPath, "-ic", command)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read interactive %s history: %w", shellName, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make([]string, 0, 512)
	for scanner.Scan() {
		line := normalizeHistoryLine(scanner.Text(), shellName)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed while reading interactive %s history: %w", shellName, err)
	}

	return lines, nil
}

func shellHistoryCommand(shellName string) string {
	switch shellName {
	case "zsh":
		return "fc -R; fc -ln 1"
	case "bash":
		return "HISTTIMEFORMAT= history"
	default:
		return ""
	}
}

func readFirstAvailableHistory(historyFiles []historyFile) ([]string, error) {
	for _, historyFile := range historyFiles {
		lines, err := readHistoryFile(historyFile.path, historyFile.format)
		if err != nil {
			return nil, err
		}
		if len(lines) > 0 {
			return lines, nil
		}
	}

	return nil, nil
}

func dedupeKeepLast(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))

	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	return out
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
		line = strings.TrimSpace(stripLeadingHistoryNumber(line))
	}

	line = strings.Join(strings.Fields(line), " ")
	return strings.TrimSpace(line)
}

func stripLeadingHistoryNumber(line string) string {
	line = strings.TrimLeft(line, " \t")
	if line == "" || line[0] < '0' || line[0] > '9' {
		return line
	}

	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}

	if i >= len(line) || (line[i] != ' ' && line[i] != '\t') {
		return line
	}

	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}

	return line[i:]
}
