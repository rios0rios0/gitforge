package domain

import (
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// deduplicationOverlapThreshold is the minimum overlap ratio to consider two entries as duplicates.
const deduplicationOverlapThreshold = 0.6

// stopWords are common words stripped during tokenization for similarity comparison.
//
//nolint:gochecknoglobals // constant-like lookup table
var stopWords = map[string]bool{
	"the": true, "to": true, "and": true, "all": true, "their": true,
	"its": true, "a": true, "an": true, "of": true, "in": true,
	"for": true, "with": true, "from": true, "by": true, "on": true,
	"is": true, "was": true, "are": true, "were": true, "be": true,
	"been": true, "being": true, "has": true, "have": true, "had": true,
	"that": true, "this": true, "it": true, "as": true,
}

// backtickPattern matches backtick-wrapped content.
var backtickPattern = regexp.MustCompile("`[^`]*`")

// changelogVersionPattern matches semver-like version numbers (e.g., 1.26.0, v2.3.1).
var changelogVersionPattern = regexp.MustCompile(`v?\d+\.\d+(?:\.\d+)?`)

// normalizeEntry strips a changelog entry down to its semantic core for comparison.
func normalizeEntry(entry string) string {
	s := strings.TrimSpace(entry)
	s = strings.TrimPrefix(s, "- ")
	s = backtickPattern.ReplaceAllString(s, "")
	s = changelogVersionPattern.ReplaceAllString(s, "")
	s = strings.ToLower(s)
	return strings.Join(strings.Fields(s), " ")
}

// tokenize splits a normalized entry into significant words, removing stop words.
func tokenize(normalized string) []string {
	words := strings.Fields(normalized)
	var tokens []string
	for _, w := range words {
		if !stopWords[w] && len(w) > 1 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// extractMaxVersion finds the highest semver version mentioned in an entry's raw text.
func extractMaxVersion(entry string) *semver.Version {
	matches := changelogVersionPattern.FindAllString(entry, -1)
	var maxVer *semver.Version
	for _, m := range matches {
		v, err := semver.NewVersion(m)
		if err != nil {
			continue
		}
		if maxVer == nil || v.GreaterThan(maxVer) {
			maxVer = v
		}
	}
	return maxVer
}

// overlapRatio computes the token overlap ratio between two token slices.
func overlapRatio(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}

	intersection := 0
	for _, t := range b {
		if set[t] {
			intersection++
		}
	}

	minLen := min(len(b), len(a))

	return float64(intersection) / float64(minLen)
}

// DeduplicateEntries removes duplicate and semantically overlapping changelog entries.
func DeduplicateEntries(entries []string) []string {
	if len(entries) <= 1 {
		return entries
	}

	seen := make(map[string]bool, len(entries))
	var unique []string
	for _, e := range entries {
		normalized := strings.TrimSpace(e)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		unique = append(unique, e)
	}

	if len(unique) <= 1 {
		return unique
	}

	type entryInfo struct {
		raw    string
		tokens []string
		ver    *semver.Version
	}

	infos := make([]entryInfo, len(unique))
	for i, e := range unique {
		infos[i] = entryInfo{
			raw:    e,
			tokens: tokenize(normalizeEntry(e)),
			ver:    extractMaxVersion(e),
		}
	}

	removed := make(map[int]bool)

	for i := range infos {
		if removed[i] {
			continue
		}
		for j := i + 1; j < len(infos); j++ {
			if removed[j] {
				continue
			}

			ratio := overlapRatio(infos[i].tokens, infos[j].tokens)
			if ratio < deduplicationOverlapThreshold {
				continue
			}

			loser := pickLoser(infos[i], infos[j], i, j)
			removed[loser] = true
		}
	}

	var result []string
	for i, info := range infos {
		if !removed[i] {
			result = append(result, info.raw)
		}
	}
	return result
}

// pickLoser decides which of two overlapping entries to remove.
func pickLoser(a, b struct {
	raw    string
	tokens []string
	ver    *semver.Version
}, idxA, idxB int,
) int {
	switch {
	case a.ver != nil && b.ver != nil:
		if a.ver.GreaterThan(b.ver) {
			return idxB
		}
		if b.ver.GreaterThan(a.ver) {
			return idxA
		}
	case a.ver != nil:
		return idxB
	case b.ver != nil:
		return idxA
	}

	if len(a.raw) != len(b.raw) {
		if len(a.raw) > len(b.raw) {
			return idxB
		}
		return idxA
	}

	return idxB
}
