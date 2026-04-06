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

func GetFuzzyScoreList(commandHistory []string, query string) []ScoredCommand {
	query = strings.TrimSpace(query)

	bestByCommand := make(map[string]ScoredCommand, len(commandHistory))

	for i, cmd := range commandHistory {
		score := 0
		if query != "" {
			score = FuzzyScore(query, cmd)
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
