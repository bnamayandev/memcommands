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

func MatchPositions(query, target string) []int {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	qb := []byte(query)
	tb := []byte(target)
	positions := make([]int, 0, len(qb))
	qi := 0

	for ti := 0; ti < len(tb) && qi < len(qb); ti++ {
		if lc(tb[ti]) == lc(qb[qi]) {
			positions = append(positions, ti)
			qi++
		}
	}

	if qi == len(qb) {
		return positions
	}
	return nil
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

func NormalizeCommandKey(s string) string {
	return normalizeCommandKey(s)
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

	// User aliases label the whole command line, so they match as standalone
	// search terms rather than as a command prefix.
	for _, label := range aliases.ByFullCommand[normalizeCommandKey(command)] {
		variants = append(variants, label)
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

func bestVariantScore(query string, variants []string) int {
	best := -1

	for _, variant := range variants {
		score := FuzzyScore(query, variant)
		if score > best {
			best = score
		}
	}

	return best
}

// corpusEntry precomputes a command's match variants so search doesn't rebuild
// them on every keystroke.
type corpusEntry struct {
	command  string
	index    int
	variants []string
}

// Corpus is a search-ready, de-duplicated view of history; rebuild only when the
// alias index changes.
type Corpus struct {
	entries []corpusEntry
}

func NewCorpus(commandHistory []string, aliases AliasIndex) *Corpus {
	entries := make([]corpusEntry, 0, len(commandHistory))
	posByKey := make(map[string]int, len(commandHistory))

	for i, cmd := range commandHistory {
		command := strings.Join(strings.Fields(strings.TrimSpace(cmd)), " ")
		if command == "" {
			continue
		}

		entry := corpusEntry{
			command:  command,
			index:    i,
			variants: commandVariants(command, aliases),
		}

		key := normalizeCommandKey(command)
		if pos, ok := posByKey[key]; ok {
			// Keep the most recent occurrence (larger index wins recency ties).
			entries[pos] = entry
			continue
		}
		posByKey[key] = len(entries)
		entries = append(entries, entry)
	}

	return &Corpus{entries: entries}
}

// Search returns matches ordered by score then recency; an empty query returns
// everything most-recent-first.
func (c *Corpus) Search(query string) []ScoredCommand {
	query = strings.TrimSpace(query)

	out := make([]ScoredCommand, 0, len(c.entries))
	for _, entry := range c.entries {
		score := 0
		if query != "" {
			score = bestVariantScore(query, entry.variants)
			if score < 0 {
				continue
			}
		}

		out = append(out, ScoredCommand{
			Score:   score,
			Command: entry.command,
			Index:   entry.index,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Index > out[j].Index
		}
		return out[i].Score > out[j].Score
	})

	return out
}

// GetFuzzyScoreList builds a one-off corpus and searches it; the TUI uses a
// persistent Corpus instead.
func GetFuzzyScoreList(commandHistory []string, query string, aliases AliasIndex) []ScoredCommand {
	return NewCorpus(commandHistory, aliases).Search(query)
}
