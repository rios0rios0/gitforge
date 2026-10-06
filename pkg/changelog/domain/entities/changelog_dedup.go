package entities

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// deduplicationOverlapThreshold is the minimum overlap ratio to consider two entries as duplicates.
const deduplicationOverlapThreshold = 0.9

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

// codeVersionPattern matches a code span that is a version rather than an
// identifier: "1.2.3", "v0.21.0", "3.13-slim", "2024.2.2".
var codeVersionPattern = regexp.MustCompile(`^v?\d`)

// normalizeEntry strips a changelog entry down to its semantic core for comparison.
func normalizeEntry(entry string) string {
	s := strings.TrimSpace(entry)
	s = strings.TrimPrefix(s, "- ")
	s = backtickPattern.ReplaceAllStringFunc(s, func(match string) string {
		return match[1 : len(match)-1]
	})
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

// codeIdentifiers returns the code spans an entry names, leaving out the ones
// that are versions, as one comparable key: lower-cased, sorted and joined.
//
// Two entries naming different identifiers state different facts however many
// words they share. The token overlap below is measured against the shorter
// entry, so a one-library statement whose words all appear in a longer
// statement about several libraries scored as its duplicate, and whichever lost
// took a library out of the release notes. Requiring the same identifiers
// before the words are even compared keeps the overlap rule for what it is good
// at -- the same statement reworded, or restated with a newer version.
func codeIdentifiers(entry string) string {
	var identifiers []string
	for _, span := range backtickPattern.FindAllString(entry, -1) {
		content := strings.ToLower(strings.TrimSpace(span[1 : len(span)-1]))
		if content == "" || codeVersionPattern.MatchString(content) {
			continue
		}
		identifiers = append(identifiers, content)
	}
	sort.Strings(identifiers)
	return strings.Join(identifiers, "\x00")
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

// dedupEntry is one entry as DeduplicateEntries compares it.
type dedupEntry struct {
	raw         string
	tokens      []string
	ver         *semver.Version
	identifiers string
}

// DeduplicateEntries removes duplicate and semantically overlapping changelog
// entries. Two entries are compared word by word only when they name the same
// code identifiers -- see [codeIdentifiers].
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

	infos := make([]dedupEntry, len(unique))
	for i, e := range unique {
		infos[i] = dedupEntry{
			raw:         e,
			tokens:      tokenize(normalizeEntry(e)),
			ver:         extractMaxVersion(e),
			identifiers: codeIdentifiers(e),
		}
	}

	removed := restatedEntries(infos)

	var result []string
	for i, info := range infos {
		if !removed[i] {
			result = append(result, info.raw)
		}
	}
	return result
}

// restatedEntries returns the indexes of the entries another entry restates,
// keeping of each pair the one pickLoser prefers.
func restatedEntries(infos []dedupEntry) map[int]bool {
	removed := make(map[int]bool)
	for i := range infos {
		if removed[i] {
			continue
		}
		for j := i + 1; j < len(infos); j++ {
			if !removed[j] && isRestatement(infos[i], infos[j]) {
				removed[pickLoser(infos[i], infos[j], i, j)] = true
			}
		}
	}
	return removed
}

// isRestatement reports whether two entries make the same statement: they name
// the same code identifiers and their words overlap past the threshold.
func isRestatement(a, b dedupEntry) bool {
	return a.identifiers == b.identifiers &&
		overlapRatio(a.tokens, b.tokens) >= deduplicationOverlapThreshold
}

// pickLoser decides which of two overlapping entries to remove.
func pickLoser(a, b dedupEntry, idxA, idxB int) int {
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
