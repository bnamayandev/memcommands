package core

import (
	"sort"
	"strings"
	"unicode"
)

func lc(b byte) byte {
	return byte(unicode.ToLower(rune(b)))
}

func FuzzyScore(query, target string) int {
	score := 0
	qi := 0
	lastMatch := -1
	consecutive := 0

	qb := []byte(query)
	tb := []byte(target)

	for ti := 0; ti < len(tb) && qi < len(qb); ti++ {
		score -= 1

		if lc(tb[ti]) == lc(qb[qi]) {
			score += 5

			if lastMatch == ti-1 {
				consecutive++
				score += consecutive * 15
			} else {
				consecutive = 0
				gap := 0
				if lastMatch == -1 {
					gap = ti
				} else {
					gap = ti - lastMatch - 1
				}
				score -= gap
			}

			if ti == 0 || tb[ti-1] == ' ' || tb[ti-1] == '/' || tb[ti-1] == '-' {
				score += 10
			}

			lastMatch = ti
			qi++
		}
	}

	if qi == len(qb) {
		return score
	}
	return -1
}

type ScoredCommand struct {
	Score   int
	Command string
	Index   int
}

func normalizeCommandKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ToLower(s)
	return s
}

func commandVariants(command string, aliases AliasIndex) []string {
	command = strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
	if command == "" {
		return nil
	}

	variants := []string{command}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return variants
	}

	rest := ""
	if len(fields) > 1 {
		rest = " " + strings.Join(fields[1:], " ")
	}

	first := fields[0]
	if expanded, ok := aliases.ByAlias[first]; ok {
		variants = append(variants, expanded+rest)
	}

	for _, alias := range aliases.ByCommand[first] {
		variants = append(variants, alias+rest)
	}

	return dedupeStrings(variants)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out
}

func bestCommandScore(query, command string, aliases AliasIndex) int {
	best := -1

	for _, variant := range commandVariants(command, aliases) {
		score := FuzzyScore(query, variant)
		if score > best {
			best = score
		}
	}

	return best
}

func GetFuzzyScoreList(commandHistory []string, query string, aliases AliasIndex) []ScoredCommand {
	query = strings.TrimSpace(query)

	bestByCommand := make(map[string]ScoredCommand, len(commandHistory))

	for i, cmd := range commandHistory {
		score := 0
		if query != "" {
			score = bestCommandScore(query, cmd, aliases)
			if score < 0 {
				continue
			}
		}

		key := normalizeCommandKey(cmd)
		candidate := ScoredCommand{
			Score:   score,
			Command: strings.Join(strings.Fields(strings.TrimSpace(cmd)), " "),
			Index:   i,
		}

		existing, ok := bestByCommand[key]
		if !ok || candidate.Score > existing.Score || (candidate.Score == existing.Score && candidate.Index > existing.Index) {
			bestByCommand[key] = candidate
		}
	}

	out := make([]ScoredCommand, 0, len(bestByCommand))
	for _, scored := range bestByCommand {
		out = append(out, scored)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Index > out[j].Index
		}
		return out[i].Score > out[j].Score
	})

	return out
}
