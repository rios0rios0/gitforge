package entities

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
)

// Process processes the changelog lines and returns the next version and the new content.
func (c *Changelog) Process() (*semver.Version, []string, error) {
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	latestVersion, err := c.FindLatestVersion()
	isNewChangelog := errors.Is(err, ErrNoVersionFoundInChangelog)
	if err != nil && !isNewChangelog {
		log.Errorf("Error finding latest version: %v", err)
		return nil, nil, err
	}

	if isNewChangelog {
		log.Infof("No previous version found, will release as %s", InitialReleaseVersion)
		return c.ProcessNew()
	}

	log.Infof("Previous version: %s", latestVersion)
	nextVersion := *latestVersion

	for _, line := range c.lines {
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

// ProcessNew handles changelogs that only have [Unreleased] section.
func (c *Changelog) ProcessNew() (*semver.Version, []string, error) {
	var newContent []string
	var unreleasedSection []string
	unreleased := false

	for _, line := range c.lines {
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
