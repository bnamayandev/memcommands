package core

import (
	"sort"
	"strings"
)

// Scoring weights for the optimal-alignment matcher, modeled on fzf: a flat
// per-character reward, affine gap penalties, and bonuses that favor matches at
// word boundaries and in consecutive runs.
const (
	scoreMatch       = 16
	scoreGapStart    = -3
	scoreGapExt      = -1
	bonusBoundary    = 8
	bonusCamel       = 7
	bonusConsecutive = 4
	bonusFirstChar   = 2
	negInf           = -(1 << 30)
)

func lc(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 'a' - 'A'
	}
	return b
}

func lowerBytes(s string) []byte {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		b[i] = lc(s[i])
	}
	return b
}

type charClass uint8

const (
	classWhite charClass = iota
	classNonWord
	classDigit
	classLower
	classUpper
)

func classOf(b byte) charClass {
	switch {
	case b == ' ' || b == '\t':
		return classWhite
	case b >= 'a' && b <= 'z':
		return classLower
	case b >= 'A' && b <= 'Z':
		return classUpper
	case b >= '0' && b <= '9':
		return classDigit
	default:
		return classNonWord
	}
}

// boundaryBonus rewards a match that begins a word, a camelCase hump, or a
// number, so query characters landing on natural starts rank higher.
func boundaryBonus(prev, curr charClass) int {
	switch {
	case prev == classWhite && curr != classWhite:
		return bonusBoundary
	case prev == classNonWord && curr != classNonWord:
		return bonusBoundary
	case prev == classLower && curr == classUpper:
		return bonusCamel
	case prev != classDigit && curr == classDigit:
		return bonusCamel
	default:
		return 0
	}
}

// matchTarget is a search string precomputed once: lowercased bytes for
// case-insensitive comparison and a per-position boundary bonus.
type matchTarget struct {
	lower []byte
	bonus []int
}

func prepareTarget(s string) matchTarget {
	n := len(s)
	t := matchTarget{lower: make([]byte, n), bonus: make([]int, n)}
	prev := classWhite
	for i := 0; i < n; i++ {
		c := s[i]
		t.lower[i] = lc(c)
		curr := classOf(c)
		t.bonus[i] = boundaryBonus(prev, curr)
		prev = curr
	}
	return t
}

// fuzzyBounds reports the smallest [start, end) window of target that can hold
// every query byte as an ordered subsequence, or ok=false when it can't. It
// doubles as the cheap reject that skips the DP for non-matching targets.
func fuzzyBounds(q, lower []byte) (start, end int, ok bool) {
	qi := 0
	for i := 0; i < len(lower); i++ {
		if lower[i] == q[qi] {
			if qi == 0 {
				start = i
			}
			qi++
			if qi == len(q) {
				break
			}
		}
	}
	if qi < len(q) {
		return 0, 0, false
	}

	last := q[len(q)-1]
	for i := len(lower) - 1; i >= start; i-- {
		if lower[i] == last {
			end = i + 1
			break
		}
	}
	return start, end, true
}

// align computes the optimal case-insensitive subsequence alignment of q within
// target. It returns the score and, when wantPos is set, the matched byte
// positions; ok is false when q isn't a subsequence of target.
func align(q []byte, t matchTarget, wantPos bool) (score int, positions []int, ok bool) {
	m := len(q)
	if m == 0 {
		return 0, nil, false
	}
	sidx, eidx, found := fuzzyBounds(q, t.lower)
	if !found {
		return 0, nil, false
	}
	w := eidx - sidx

	prev := make([]int, w)
	curr := make([]int, w)
	var parents [][]int
	if wantPos {
		parents = make([][]int, m)
	}

	for i := 0; i < m; i++ {
		qc := q[i]
		var par []int
		if wantPos {
			par = make([]int, w)
			parents[i] = par
		}

		// runBest tracks the best affine-gap source over earlier columns of the
		// previous row, folded in one column at a time as it becomes eligible.
		runBest := negInf
		runArg := -1
		for c := 0; c < w; c++ {
			if i > 0 && c >= 2 && prev[c-2] > negInf {
				if a := prev[c-2] - scoreGapExt*(c-2); a > runBest {
					runBest = a
					runArg = c - 2
				}
			}

			p := sidx + c
			if t.lower[p] != qc {
				curr[c] = negInf
				if wantPos {
					par[c] = -1
				}
				continue
			}

			if i == 0 {
				curr[c] = scoreMatch + t.bonus[p]*bonusFirstChar
				if wantPos {
					par[c] = -1
				}
				continue
			}

			best := negInf
			bestK := -1
			if c >= 1 && prev[c-1] > negInf {
				if v := prev[c-1] + bonusConsecutive; v > best {
					best = v
					bestK = c - 1
				}
			}
			if runBest > negInf {
				if v := runBest + scoreGapStart + scoreGapExt*(c-2); v > best {
					best = v
					bestK = runArg
				}
			}

			if best > negInf {
				curr[c] = scoreMatch + t.bonus[p] + best
				if wantPos {
					par[c] = bestK
				}
			} else {
				curr[c] = negInf
				if wantPos {
					par[c] = -1
				}
			}
		}

		prev, curr = curr, prev
	}

	// After the final swap, prev holds the last query row.
	best := negInf
	bestCol := -1
	for c := 0; c < w; c++ {
		if prev[c] > best {
			best = prev[c]
			bestCol = c
		}
	}
	if bestCol < 0 {
		return 0, nil, false
	}

	if wantPos {
		positions = make([]int, m)
		col := bestCol
		for i := m - 1; i >= 0; i-- {
			positions[i] = sidx + col
			col = parents[i][col]
		}
	}

	return best, positions, true
}

// FuzzyScore returns the optimal alignment score of query within target, or -1
// when target doesn't contain query as a fuzzy subsequence.
func FuzzyScore(query, target string) int {
	q := lowerBytes(strings.TrimSpace(query))
	if len(q) == 0 {
		return 0
	}
	score, _, ok := align(q, prepareTarget(target), false)
	if !ok {
		return -1
	}
	return score
}

// MatchPositions returns the byte offsets in target that make up the optimal
// match for query, or nil when there is no match.
func MatchPositions(query, target string) []int {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	_, positions, ok := align(lowerBytes(query), prepareTarget(target), true)
	if !ok {
		return nil
	}
	return positions
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

func bestVariantScore(q []byte, variants []matchTarget) (int, bool) {
	best := negInf
	for i := range variants {
		if score, _, ok := align(q, variants[i], false); ok && score > best {
			best = score
		}
	}
	if best == negInf {
		return 0, false
	}
	return best, true
}

// corpusEntry precomputes a command's match variants so search doesn't rebuild
// them on every keystroke.
type corpusEntry struct {
	command  string
	index    int
	variants []matchTarget
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

		vs := commandVariants(command, aliases)
		prepared := make([]matchTarget, len(vs))
		for j, v := range vs {
			prepared[j] = prepareTarget(v)
		}

		entry := corpusEntry{
			command:  command,
			index:    i,
			variants: prepared,
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
	var q []byte
	if query != "" {
		q = lowerBytes(query)
	}

	out := make([]ScoredCommand, 0, len(c.entries))
	for i := range c.entries {
		entry := &c.entries[i]
		score := 0
		if len(q) > 0 {
			s, ok := bestVariantScore(q, entry.variants)
			if !ok {
				continue
			}
			score = s
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
