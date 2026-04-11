package core

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

type AliasIndex struct {
	ByAlias   map[string]string
	ByCommand map[string][]string
}

func LoadAliases() AliasIndex {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return AliasIndex{}
	}

	cmd := exec.Command(shell, "-ic", "alias")
	output, err := cmd.Output()
	if err != nil {
		return AliasIndex{}
	}

	byAlias := make(map[string]string)
	byCommand := make(map[string][]string)

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		name, expansion, ok := parseAliasLine(scanner.Text())
		if !ok {
			continue
		}

		expansion = strings.Join(strings.Fields(expansion), " ")
		if expansion == "" {
			continue
		}

		byAlias[name] = expansion

		fields := strings.Fields(expansion)
		if len(fields) == 0 {
			continue
		}

		command := fields[0]
		aliases := byCommand[command]
		if !containsString(aliases, name) {
			byCommand[command] = append(aliases, name)
		}
	}

	return AliasIndex{
		ByAlias:   byAlias,
		ByCommand: byCommand,
	}
}

func parseAliasLine(line string) (name, expansion string, ok bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "alias ")
	if line == "" {
		return "", "", false
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	name = strings.TrimSpace(parts[0])
	expansion = strings.TrimSpace(parts[1])
	expansion = strings.Trim(expansion, `"'`)

	if name == "" || expansion == "" {
		return "", "", false
	}

	return name, expansion, true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func ExpandAliasCommand(command string, aliases AliasIndex) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}

	expanded, ok := aliases.ByAlias[fields[0]]
	if !ok {
		return strings.Join(fields, " ")
	}

	if len(fields) == 1 {
		return expanded
	}

	return expanded + " " + strings.Join(fields[1:], " ")
}
