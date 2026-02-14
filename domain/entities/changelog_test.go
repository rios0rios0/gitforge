package entities_test

import (
	"strings"
	"testing"

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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version.String() != "1.2.0" {
			t.Errorf("expected 1.2.0, got %s", version.String())
		}
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
		if err == nil {
			t.Fatal("expected error, got nil")
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if empty {
			t.Error("expected false, got true")
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !empty {
			t.Error("expected true, got false")
		}
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
		if len(result) != 2 {
			t.Errorf("expected 2 entries, got %d: %v", len(result), result)
		}
	})

	t.Run("should return single entry unchanged", func(t *testing.T) {
		t.Parallel()

		// given
		entries := []string{"- added feature A"}

		// when
		result := entities.DeduplicateEntries(entries)

		// then
		if len(result) != 1 {
			t.Errorf("expected 1 entry, got %d", len(result))
		}
	})

	t.Run("should return nil for empty input", func(t *testing.T) {
		t.Parallel()

		// given
		var entries []string

		// when
		result := entities.DeduplicateEntries(entries)

		// then
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
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
		if !strings.Contains(result, "- new entry") {
			t.Error("expected new entry in result")
		}
		if !strings.Contains(result, "- existing entry") {
			t.Error("expected existing entry to remain")
		}
	})

	t.Run("should create Changed section when missing", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2024-01-01\n"
		entries := []string{"- new entry"}

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		if !strings.Contains(result, "### Changed") {
			t.Error("expected Changed section to be created")
		}
		if !strings.Contains(result, "- new entry") {
			t.Error("expected new entry in result")
		}
	})

	t.Run("should return content unchanged when no Unreleased section", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [1.0.0] - 2024-01-01\n"
		entries := []string{"- new entry"}

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		if result != content {
			t.Error("expected content to remain unchanged")
		}
	})

	t.Run("should return content unchanged when entries are empty", func(t *testing.T) {
		t.Parallel()

		// given
		content := "# Changelog\n\n## [Unreleased]\n"
		var entries []string

		// when
		result := entities.InsertChangelogEntry(content, entries)

		// then
		if result != content {
			t.Error("expected content to remain unchanged")
		}
	})
}
