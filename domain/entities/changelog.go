package entities

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
)

// InitialReleaseVersion is the version used when no version is found in the changelog.
// When a changelog only has [Unreleased] section, we bump directly to 1.0.0.
const InitialReleaseVersion = "1.0.0"

var (
	ErrNoVersionFoundInChangelog  = errors.New("no version found in the changelog")
	ErrNoChangesFoundInUnreleased = errors.New("no changes found in the unreleased section")
)

// --- Full changelog processing (from autobump) ---

// IsChangelogUnreleasedEmpty checks whether the unreleased section of the changelog is empty.
func IsChangelogUnreleasedEmpty(lines []string) (bool, error) {
	latestVersion, err := FindLatestVersion(lines)
	noVersionFound := errors.Is(err, ErrNoVersionFoundInChangelog)
	if err != nil && !noVersionFound {
		return true, err
	}

	unreleased := false
	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		} else if !noVersionFound &&
			strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
			unreleased = false
		}

		if unreleased {
			re := regexp.MustCompile(`^\s*-\s*[^ ]+`)
			if match := re.MatchString(line); match {
				return false, nil
			}
		}
	}

	return true, nil
}

// FindLatestVersion finds the latest version in the changelog lines.
func FindLatestVersion(lines []string) (*semver.Version, error) {
	versionRegex := regexp.MustCompile(`^\s*##\s*\[([^\]]+)\]`)

	var latestVersion *semver.Version
	for _, line := range lines {
		if versionMatch := versionRegex.FindStringSubmatch(line); versionMatch != nil {
			if versionMatch[1] == "Unreleased" {
				continue
			}

			version, err := semver.NewVersion(versionMatch[1])
			if err != nil {
				log.Errorf("Error parsing version '%s': %v", versionMatch[1], err)
				return nil, fmt.Errorf("error parsing version '%s': %w", versionMatch[1], err)
			}

			if latestVersion == nil || version.GreaterThan(latestVersion) {
				latestVersion = version
			}
		}
	}

	if latestVersion == nil {
		return nil, ErrNoVersionFoundInChangelog
	}

	return latestVersion, nil
}

// ProcessChangelog processes the changelog lines and returns the next version and the new content.
func ProcessChangelog(lines []string) (*semver.Version, []string, error) {
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	latestVersion, err := FindLatestVersion(lines)
	isNewChangelog := errors.Is(err, ErrNoVersionFoundInChangelog)
	if err != nil && !isNewChangelog {
		log.Errorf("Error finding latest version: %v", err)
		return nil, nil, err
	}

	if isNewChangelog {
		log.Infof("No previous version found, will release as %s", InitialReleaseVersion)
		return ProcessNewChangelog(lines)
	}

	log.Infof("Previous version: %s", latestVersion)
	nextVersion := *latestVersion

	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		} else if strings.HasPrefix(line, fmt.Sprintf("## [%s]", latestVersion.String())) {
			unreleased = false
			if len(unreleasedSection) > 0 {
				var updatedSection []string
				var updatedVersion *semver.Version
				updatedSection, updatedVersion, err = UpdateSection(unreleasedSection, nextVersion)
				if err != nil {
					log.Errorf("Error updating section: %v", err)
					return nil, nil, err
				}
				newContent = append(newContent, updatedSection...)
				unreleasedSection = nil
				nextVersion = *updatedVersion
			}
		}

		if unreleased {
			unreleasedSection = append(unreleasedSection, line)
		} else {
			newContent = append(newContent, line)
		}
	}

	log.Infof("Next calculated version: %s", nextVersion)
	return &nextVersion, newContent, nil
}

// ProcessNewChangelog handles changelogs that only have [Unreleased] section.
func ProcessNewChangelog(lines []string) (*semver.Version, []string, error) {
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	for _, line := range lines {
		if strings.Contains(line, "[Unreleased]") {
			unreleased = true
		}

		if unreleased {
			unreleasedSection = append(unreleasedSection, line)
		} else {
			newContent = append(newContent, line)
		}
	}

	initialVersion, _ := semver.NewVersion(InitialReleaseVersion)

	if len(unreleasedSection) > 0 {
		FixSectionHeadings(unreleasedSection)

		sections := map[string]*[]string{
			"Added":      {},
			"Changed":    {},
			"Deprecated": {},
			"Removed":    {},
			"Fixed":      {},
			"Security":   {},
		}
		var currentSection *[]string
		majorChanges, minorChanges, patchChanges := 0, 0, 0
		ParseUnreleasedIntoSections(
			unreleasedSection, sections, currentSection,
			&majorChanges, &minorChanges, &patchChanges,
		)

		for _, section := range sections {
			*section = DeduplicateEntries(*section)
		}

		hasContent := false
		for _, section := range sections {
			if len(*section) > 0 {
				hasContent = true
				break
			}
		}

		if hasContent {
			newSection := MakeNewSections(sections, *initialVersion)
			newContent = append(newContent, newSection...)
		} else {
			newSection := MakeNewSectionsFromUnreleased(unreleasedSection, *initialVersion)
			newContent = append(newContent, newSection...)
		}
	}

	log.Infof("Next calculated version: %s", InitialReleaseVersion)
	return initialVersion, newContent, nil
}

// MakeNewSectionsFromUnreleased creates new section contents for initial release.
func MakeNewSectionsFromUnreleased(unreleasedSection []string, version semver.Version) []string {
	var newSection []string

	newSection = append(newSection, "## [Unreleased]")
	newSection = append(newSection, "")
	newSection = append(
		newSection,
		fmt.Sprintf("## [%s] - %s", version.String(), time.Now().Format("2006-01-02")),
	)
	newSection = append(newSection, "")

	for _, line := range unreleasedSection {
		if !strings.Contains(line, "[Unreleased]") {
			newSection = append(newSection, line)
		}
	}

	return newSection
}

// FixSectionHeadings fixes the section headings in the unreleased section.
func FixSectionHeadings(unreleasedSection []string) {
	re := regexp.MustCompile(`(?i)^\s*#+\s*(Added|Changed|Deprecated|Removed|Fixed|Security)`)
	for i, line := range unreleasedSection {
		if re.MatchString(line) {
			correctedLine := "### " + strings.TrimSpace(strings.ReplaceAll(line, "#", ""))
			unreleasedSection[i] = correctedLine
		}
	}
}

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

// recountChanges re-counts major/minor/patch changes from deduplicated sections.
func recountChanges(sections map[string]*[]string) (int, int, int) {
	major, minor, patch := 0, 0, 0
	for key, section := range sections {
		for _, line := range *section {
			switch {
			case strings.HasPrefix(line, "- **BREAKING CHANGE:**"):
				major++
			case key == "Added":
				minor++
			default:
				patch++
			}
		}
	}
	return major, minor, patch
}

// MakeNewSections creates new section contents for the beginning of the CHANGELOG file.
func MakeNewSections(
	sections map[string]*[]string,
	nextVersion semver.Version,
) []string {
	var newSection []string
	newSection = append(newSection, "## [Unreleased]")
	newSection = append(newSection, "")
	newSection = append(
		newSection,
		fmt.Sprintf("## [%s] - %s", nextVersion.String(), time.Now().Format("2006-01-02")),
	)
	newSection = append(newSection, "")

	keys := []string{"Added", "Changed", "Deprecated", "Fixed", "Removed", "Security"}
	for _, key := range keys {
		section := sections[key]

		if len(*section) > 0 {
			newSection = append(newSection, "### "+key)
			newSection = append(newSection, "")
			newSection = append(newSection, *section...)
			newSection = append(newSection, "")
		}
	}
	return newSection
}

// ParseUnreleasedIntoSections parses the unreleased section into change type sections.
func ParseUnreleasedIntoSections(
	unreleasedSection []string,
	sections map[string]*[]string,
	currentSection *[]string,
	majorChanges, minorChanges, patchChanges *int,
) {
	for _, line := range unreleasedSection {
		trimmedLine := strings.TrimSpace(line)

		for header := range sections {
			if strings.HasPrefix(trimmedLine, "### "+header) {
				currentSection = sections[header]
			}
		}

		if currentSection != nil && trimmedLine != "" && trimmedLine != "-" &&
			!strings.HasPrefix(trimmedLine, "##") {
			*currentSection = append(*currentSection, line)

			switch {
			case strings.HasPrefix(line, "- **BREAKING CHANGE:**"):
				*majorChanges++
			case currentSection == sections["Added"]:
				*minorChanges++
			default:
				*patchChanges++
			}
		}
	}
}

// UpdateSection updates the unreleased section and calculates the next version.
func UpdateSection(
	unreleasedSection []string,
	nextVersion semver.Version,
) ([]string, *semver.Version, error) {
	FixSectionHeadings(unreleasedSection)

	sections := map[string]*[]string{
		"Added":      {},
		"Changed":    {},
		"Deprecated": {},
		"Removed":    {},
		"Fixed":      {},
		"Security":   {},
	}

	var currentSection *[]string
	majorChanges, minorChanges, patchChanges := 0, 0, 0

	ParseUnreleasedIntoSections(
		unreleasedSection,
		sections,
		currentSection,
		&majorChanges,
		&minorChanges,
		&patchChanges,
	)

	for _, section := range sections {
		*section = DeduplicateEntries(*section)
	}
	majorChanges, minorChanges, patchChanges = recountChanges(sections)

	if majorChanges == 0 && minorChanges == 0 && patchChanges == 0 {
		return nil, nil, ErrNoChangesFoundInUnreleased
	}

	switch {
	case majorChanges > 0:
		nextVersion = nextVersion.IncMajor()
	case minorChanges > 0:
		nextVersion = nextVersion.IncMinor()
	case patchChanges > 0:
		nextVersion = nextVersion.IncPatch()
	}

	for _, section := range sections {
		sort.Strings(*section)
	}

	newSection := MakeNewSections(sections, nextVersion)
	return newSection, &nextVersion, nil
}

// --- Changelog entry insertion (from autoupdate) ---

const (
	unreleasedHeading = "## [Unreleased]"
	changedSubheading = "### Changed"
	h2Prefix          = "## ["
	bulletPrefix      = "- "
)

// InsertChangelogEntry inserts one or more bullet entries into the
// "## [Unreleased]" / "### Changed" section of a Keep-a-Changelog
// formatted string.
func InsertChangelogEntry(content string, entries []string) string {
	if len(entries) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")

	unreleasedIdx := findUnreleasedIndex(lines)
	if unreleasedIdx < 0 {
		return content
	}

	nextH2Idx := findNextH2Index(lines, unreleasedIdx)
	changedIdx := findChangedIndex(lines, unreleasedIdx, nextH2Idx)

	bulletLines := make([]string, 0, len(entries))
	bulletLines = append(bulletLines, entries...)

	if changedIdx >= 0 {
		insertAfter := findLastBullet(lines, changedIdx, nextH2Idx)
		lines = insertLinesAt(lines, insertAfter+1, bulletLines)
	} else {
		block := []string{"", changedSubheading, ""}
		block = append(block, bulletLines...)
		lines = insertLinesAt(lines, unreleasedIdx+1, block)
	}

	return strings.Join(lines, "\n")
}

func findUnreleasedIndex(lines []string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) == unreleasedHeading {
			return i
		}
	}
	return -1
}

func findNextH2Index(lines []string, startIdx int) int {
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), h2Prefix) {
			return i
		}
	}
	return len(lines)
}

func findChangedIndex(lines []string, startIdx, endIdx int) int {
	for i := startIdx + 1; i < endIdx; i++ {
		if strings.TrimSpace(lines[i]) == changedSubheading {
			return i
		}
	}
	return -1
}

func findLastBullet(lines []string, changedIdx, endIdx int) int {
	insertAfter := changedIdx
	for i := changedIdx + 1; i < endIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, bulletPrefix) {
			insertAfter = i
			continue
		}
		break
	}
	return insertAfter
}

func insertLinesAt(lines []string, at int, extra []string) []string {
	result := make([]string, 0, len(lines)+len(extra))
	result = append(result, lines[:at]...)
	result = append(result, extra...)
	result = append(result, lines[at:]...)
	return result
}
