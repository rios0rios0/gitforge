package entities

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

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
	ReclassifyEntriesByVerb(sections)
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

	newSection := MakeNewSections(sections, nextVersion)
	return newSection, &nextVersion, nil
}

// verbSectionMap maps leading verbs in changelog entries to the correct section name.
//
//nolint:gochecknoglobals // constant-like lookup table
var verbSectionMap = map[string]string{
	"removed":    "Removed",
	"added":      "Added",
	"fixed":      "Fixed",
	"deprecated": "Deprecated",
}

// sectionKeys defines the fixed iteration order for changelog sections.
//
//nolint:gochecknoglobals // constant-like ordered list
var sectionKeys = []string{"Added", "Changed", "Deprecated", "Fixed", "Removed", "Security"}

// ReclassifyEntriesByVerb moves entries to the correct section based on their leading verb.
// For example, an entry "- removed old feature" under "### Changed" is moved to "### Removed".
func ReclassifyEntriesByVerb(sections map[string]*[]string) {
	for _, currentKey := range sectionKeys {
		section, ok := sections[currentKey]
		if !ok {
			continue
		}

		var kept []string
		for _, entry := range *section {
			trimmed := strings.TrimSpace(entry)
			trimmed = strings.TrimPrefix(trimmed, "- ")

			if strings.HasPrefix(trimmed, "**BREAKING CHANGE:**") {
				kept = append(kept, entry)
				continue
			}

			firstWord := strings.ToLower(strings.SplitN(trimmed, " ", 2)[0])
			targetSection, hasMapping := verbSectionMap[firstWord]

			if hasMapping && targetSection != currentKey {
				if target, targetOk := sections[targetSection]; targetOk {
					*target = append(*target, entry)
					continue
				}
			}

			kept = append(kept, entry)
		}
		*section = kept
	}
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
