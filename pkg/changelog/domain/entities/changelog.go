package entities

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
func (c *Changelog) IsUnreleasedEmpty() (bool, error) {
	latestVersion, err := c.FindLatestVersion()
	noVersionFound := errors.Is(err, ErrNoVersionFoundInChangelog)
	if err != nil && !noVersionFound {
		return true, err
	}

	unreleased := false
	for _, line := range c.lines {
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
func (c *Changelog) FindLatestVersion() (*semver.Version, error) {
	versionRegex := regexp.MustCompile(`^\s*##\s*\[([^\]]+)\]`)

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
