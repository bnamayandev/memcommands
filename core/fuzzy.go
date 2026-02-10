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
}

func GetFuzzyScoreList(commandHistory []string, query string) []ScoredCommand {
	out := make([]ScoredCommand, 0, len(commandHistory))

	query = strings.TrimSpace(query)

	for _, cmd := range commandHistory {
		out = append(out, ScoredCommand{
			Score:   FuzzyScore(query, cmd),
			Command: cmd,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})

	return out
}
