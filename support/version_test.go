package support_test

import (
	"testing"

	"github.com/rios0rios0/gitforge/support"
)

func TestSortVersionsDescending(t *testing.T) {
	t.Parallel()

	t.Run("should sort versions in descending order", func(t *testing.T) {
		t.Parallel()

		// given
		versions := []string{"1.0.0", "2.1.0", "1.5.0", "3.0.0"}

		// when
		support.SortVersionsDescending(versions)

		// then
		expected := []string{"3.0.0", "2.1.0", "1.5.0", "1.0.0"}
		for i, v := range versions {
			if v != expected[i] {
				t.Errorf("index %d: expected %q, got %q", i, expected[i], v)
			}
		}
	})

	t.Run("should handle versions with v prefix", func(t *testing.T) {
		t.Parallel()

		// given
		versions := []string{"v1.0.0", "v2.0.0", "v1.5.0"}

		// when
		support.SortVersionsDescending(versions)

		// then
		if versions[0] != "v2.0.0" {
			t.Errorf("expected v2.0.0 first, got %s", versions[0])
		}
	})
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	t.Run("should add v prefix when missing", func(t *testing.T) {
		t.Parallel()

		// given
		version := "1.0.0"

		// when
		result := support.NormalizeVersion(version)

		// then
		if result != "v1.0.0" {
			t.Errorf("expected v1.0.0, got %s", result)
		}
	})

	t.Run("should keep v prefix when already present", func(t *testing.T) {
		t.Parallel()

		// given
		version := "v2.0.0"

		// when
		result := support.NormalizeVersion(version)

		// then
		if result != "v2.0.0" {
			t.Errorf("expected v2.0.0, got %s", result)
		}
	})
}
