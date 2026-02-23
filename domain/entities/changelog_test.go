package entities_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
)

func TestFindLatestVersion(t *testing.T) {
	t.Parallel()

	t.Run("should find latest version when multiple versions exist", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"## [1.2.0] - 2024-01-01",
			"## [1.1.0] - 2023-12-01",
			"## [1.0.0] - 2023-11-01",
		}

		// when
		version, err := entities.FindLatestVersion(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.2.0", version.String())
	})

	t.Run("should return error when no version found", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
		}

		// when
		_, err := entities.FindLatestVersion(lines)

		// then
		require.Error(t, err)
	})
}

func TestIsChangelogUnreleasedEmpty(t *testing.T) {
	t.Parallel()

	t.Run("should return false when unreleased has content", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Added",
			"- added new feature",
			"## [1.0.0] - 2024-01-01",
		}

		// when
		empty, err := entities.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.False(t, empty)
	})

	t.Run("should return true when unreleased is empty", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"## [1.0.0] - 2024-01-01",
		}

		// when
		empty, err := entities.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.True(t, empty)
	})
}

func TestDeduplicateEntries(t *testing.T) {
	t.Parallel()

	t.Run("should remove exact duplicates", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- added user authentication with JWT tokens",
			"- added user authentication with JWT tokens",
			"- fixed database connection pooling for PostgreSQL",
		}

		// when
		result := entities.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 2)
	})

	t.Run("should return single entry unchanged", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{"- added feature A"}

		// when
		result := entities.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
	})

	t.Run("should return nil for empty input", func(t *testing.T) {
		t.Parallel()

		// given
		var entries []string

		// when
		result := entities.DeduplicateEntries(entries)

		// then
		assert.Empty(t, result)
	})
}

func TestInsertChangelogEntry(t *testing.T) {
	t.Parallel()

	t.Run("should insert entries under existing Changed section", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [Unreleased]\n\n### Changed\n\n- existing entry\n\n## [1.0.0] - 2024-01-01\n"
		entries := []string{"- new entry"}

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		assert.Contains(t, result, "- new entry")
		assert.Contains(t, result, "- existing entry")
	})

	t.Run("should create Changed section when missing", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2024-01-01\n"
		entries := []string{"- new entry"}

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		assert.Contains(t, result, "### Changed")
		assert.Contains(t, result, "- new entry")
	})

	t.Run("should return content unchanged when no Unreleased section", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [1.0.0] - 2024-01-01\n"
		entries := []string{"- new entry"}

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		assert.Equal(t, content, result)
	})

	t.Run("should return content unchanged when entries are empty", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [Unreleased]\n"
		var entries []string

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		assert.Equal(t, content, result)
	})
}

func TestProcessChangelog(t *testing.T) {
	t.Parallel()

	t.Run("should bump patch version when only fixes are present", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Fixed",
			"- fixed a bug",
			"## [1.0.0] - 2024-01-01",
			"### Added",
			"- initial release",
		}

		// when
		version, content, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.1", version.String())
		assert.NotEmpty(t, content)
	})

	t.Run("should bump minor version when added entries are present", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Added",
			"- new feature",
			"## [1.0.0] - 2024-01-01",
			"### Added",
			"- initial release",
		}

		// when
		version, _, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", version.String())
	})

	t.Run("should bump major version when breaking changes are present", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Changed",
			"- **BREAKING CHANGE:** removed deprecated API",
			"## [1.0.0] - 2024-01-01",
			"### Added",
			"- initial release",
		}

		// when
		version, _, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", version.String())
	})

	t.Run("should handle new changelog with only unreleased section", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Added",
			"- initial feature",
		}

		// when
		version, content, err := entities.ProcessChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", version.String())
		assert.NotEmpty(t, content)
	})

	t.Run("should return error when unreleased section is empty", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"## [1.0.0] - 2024-01-01",
			"### Added",
			"- initial release",
		}

		// when
		_, _, err := entities.ProcessChangelog(lines)

		// then
		require.Error(t, err)
	})
}

func TestProcessNewChangelog(t *testing.T) {
	t.Parallel()

	t.Run("should return 1.0.0 for new changelog with content", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Added",
			"- first feature",
		}

		// when
		version, content, err := entities.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", version.String())
		assert.NotEmpty(t, content)
	})

	t.Run("should handle changelog with prefix content before unreleased", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"All notable changes to this project will be documented in this file.",
			"## [Unreleased]",
			"### Changed",
			"- some change",
		}

		// when
		version, content, err := entities.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", version.String())
		// prefix content should be preserved
		found := false
		for _, line := range content {
			if strings.Contains(line, "All notable changes") {
				found = true
			}
		}
		assert.True(t, found, "expected prefix content to be preserved")
	})

	t.Run("should handle changelog with no unreleased section", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"Some content without unreleased",
		}

		// when
		version, _, err := entities.ProcessNewChangelog(lines)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", version.String())
	})
}

func TestFixSectionHeadings(t *testing.T) {
	t.Parallel()

	t.Run("should fix section headings to ### format", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"## [Unreleased]",
			"## Added",
			"- new feature",
			"#### Changed",
			"- some change",
		}

		// when
		entities.FixSectionHeadings(lines)

		// then
		assert.Equal(t, "### Added", lines[1])
		assert.Equal(t, "### Changed", lines[3])
	})

	t.Run("should handle all standard section types", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Added",
			"# Changed",
			"# Deprecated",
			"# Removed",
			"# Fixed",
			"# Security",
		}

		// when
		entities.FixSectionHeadings(lines)

		// then
		expected := []string{
			"### Added",
			"### Changed",
			"### Deprecated",
			"### Removed",
			"### Fixed",
			"### Security",
		}
		for i, line := range lines {
			assert.Equal(t, expected[i], line, "index %d", i)
		}
	})
}

func TestUpdateSection(t *testing.T) {
	t.Parallel()

	t.Run("should return error when unreleased section has no changes", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"",
		}
		nextVersion, _ := entities.FindLatestVersion([]string{"## [1.0.0] - 2024-01-01"})

		// when
		_, _, err := entities.UpdateSection(unreleasedSection, *nextVersion)

		// then
		require.Error(t, err)
	})

	t.Run("should bump patch version for fixed changes", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"### Fixed",
			"- fixed a critical bug",
		}
		nextVersion, _ := entities.FindLatestVersion([]string{"## [2.0.0] - 2024-01-01"})

		// when
		newSection, version, err := entities.UpdateSection(unreleasedSection, *nextVersion)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.1", version.String())
		assert.NotEmpty(t, newSection)
	})

	t.Run("should bump minor version for added changes", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"### Added",
			"- new feature ABC",
		}
		nextVersion, _ := entities.FindLatestVersion([]string{"## [1.0.0] - 2024-01-01"})

		// when
		_, version, err := entities.UpdateSection(unreleasedSection, *nextVersion)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", version.String())
	})

	t.Run("should bump major version for breaking changes", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"### Changed",
			"- **BREAKING CHANGE:** removed deprecated endpoint",
		}
		nextVersion, _ := entities.FindLatestVersion([]string{"## [1.5.0] - 2024-01-01"})

		// when
		_, version, err := entities.UpdateSection(unreleasedSection, *nextVersion)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", version.String())
	})
}

func TestMakeNewSectionsFromUnreleased(t *testing.T) {
	t.Parallel()

	t.Run("should create versioned section from unreleased content", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"### Added",
			"- new feature",
		}
		version, _ := entities.FindLatestVersion([]string{"## [1.0.0] - 2024-01-01"})

		// when
		result := entities.MakeNewSectionsFromUnreleased(unreleasedSection, *version)

		// then
		require.NotEmpty(t, result)

		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "## [Unreleased]")
		assert.Contains(t, joined, "[1.0.0]")
		assert.Contains(t, joined, "### Added")
	})
}

func TestParseUnreleasedIntoSections(t *testing.T) {
	t.Parallel()

	t.Run("should parse entries into correct sections", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"### Added",
			"- new feature A",
			"### Changed",
			"- changed behavior B",
			"### Fixed",
			"- fixed bug C",
		}
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

		// when
		entities.ParseUnreleasedIntoSections(
			unreleasedSection, sections, currentSection,
			&majorChanges, &minorChanges, &patchChanges,
		)

		// then
		assert.Len(t, *sections["Added"], 1)
		assert.Len(t, *sections["Changed"], 1)
		assert.Len(t, *sections["Fixed"], 1)
		assert.Equal(t, 1, minorChanges)
		assert.Equal(t, 2, patchChanges)
	})

	t.Run("should count breaking changes as major", func(t *testing.T) {
		t.Parallel()

		// given
		unreleasedSection := []string{
			"## [Unreleased]",
			"### Changed",
			"- **BREAKING CHANGE:** removed old API",
		}
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

		// when
		entities.ParseUnreleasedIntoSections(
			unreleasedSection, sections, currentSection,
			&majorChanges, &minorChanges, &patchChanges,
		)

		// then
		assert.Equal(t, 1, majorChanges)
	})
}

func TestMakeNewSections(t *testing.T) {
	t.Parallel()

	t.Run("should create formatted sections with all change types", func(t *testing.T) {
		t.Parallel()

		// given
		sections := map[string]*[]string{
			"Added":      {"- new feature"},
			"Changed":    {"- changed behavior"},
			"Deprecated": {},
			"Removed":    {},
			"Fixed":      {"- fixed bug"},
			"Security":   {},
		}
		version, _ := entities.FindLatestVersion([]string{"## [2.0.0] - 2024-01-01"})

		// when
		result := entities.MakeNewSections(sections, *version)

		// then
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "## [Unreleased]")
		assert.Contains(t, joined, "[2.0.0]")
		assert.Contains(t, joined, "### Added")
		assert.Contains(t, joined, "### Changed")
		assert.Contains(t, joined, "### Fixed")
		// Deprecated, Removed, Security should NOT appear since they are empty
		assert.NotContains(t, joined, "### Deprecated")
	})
}

func TestFindLatestVersionWithInvalidVersion(t *testing.T) {
	t.Parallel()

	t.Run("should return error when version string is invalid", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"## [not-a-version] - 2024-01-01",
		}

		// when
		_, err := entities.FindLatestVersion(lines)

		// then
		require.Error(t, err)
	})
}

func TestIsChangelogUnreleasedEmptyWithNoVersions(t *testing.T) {
	t.Parallel()

	t.Run("should return false when unreleased has content and no version exists", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
			"### Added",
			"- new feature",
		}

		// when
		empty, err := entities.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.False(t, empty)
	})

	t.Run("should return true when unreleased is empty and no version exists", func(t *testing.T) {
		t.Parallel()

		// given
		lines := []string{
			"# Changelog",
			"## [Unreleased]",
		}

		// when
		empty, err := entities.IsChangelogUnreleasedEmpty(lines)

		// then
		require.NoError(t, err)
		assert.True(t, empty)
	})
}

func TestDeduplicateEntriesSemanticOverlap(t *testing.T) {
	t.Parallel()

	t.Run("should merge semantically overlapping entries keeping the longer one", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{
			"- updated dependency `foo` from 1.0.0 to 2.0.0",
			"- updated dependency `foo` from 1.0.0 to 3.0.0",
		}

		// when
		result := entities.DeduplicateEntries(entries)

		// then
		assert.Len(t, result, 1)
	})
}
