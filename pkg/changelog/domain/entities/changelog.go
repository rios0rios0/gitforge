package entities

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	log "github.com/sirupsen/logrus"
)

// InitialReleaseVersion is the version used when no previous release version is present in the changelog.
const InitialReleaseVersion = "0.1.0"

// versionHeadingPattern matches a "## [x]" heading and captures x: a version, or "Unreleased".
const versionHeadingPattern = `^\s*##\s*\[([^\]]+)\]`

var (
	ErrNoVersionFoundInChangelog  = errors.New("no version found in the changelog")
	ErrNoChangesFoundInUnreleased = errors.New("no changes found in the unreleased section")
)

// Changelog encapsulates a Keep-a-Changelog formatted document as lines.
type Changelog struct {
	lines []string
}

// NewChangelog creates a new Changelog from the given lines.
func NewChangelog(lines []string) *Changelog {
	return &Changelog{lines: lines}
}

// Lines returns the underlying lines of the changelog.
func (c *Changelog) Lines() []string {
	return c.lines
}

// IsUnreleasedEmpty checks whether the unreleased section of the changelog is empty.
//
// The section ends at the next "## [x]" heading, whatever x is. No version is parsed: whether
// there is anything to release does not depend on how releases are numbered, so a changelog
// whose headings are not SemVer (e.g. a fork's X.Y.Z.N) still gets an answer. Validating the
// headings is left to FindLatestVersion. The error is always nil; it is kept for compatibility.
func (c *Changelog) IsUnreleasedEmpty() (bool, error) {
	headingRegex := regexp.MustCompile(versionHeadingPattern)
	entryRegex := regexp.MustCompile(`^\s*-\s*[^ ]+`)

	unreleased := false
	for _, line := range c.lines {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "## [Unreleased]"):
			unreleased = true
		case headingRegex.MatchString(line):
			unreleased = false
		case unreleased && entryRegex.MatchString(line):
			return false, nil
		}
	}

	return true, nil
}

// FindLatestVersion finds the latest version in the changelog lines.
func (c *Changelog) FindLatestVersion() (*semver.Version, error) {
	versionRegex := regexp.MustCompile(versionHeadingPattern)

	var latestVersion *semver.Version
	for _, line := range c.lines {
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
