package support

import (
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

// SortVersionsDescending sorts a slice of version strings in descending semver order.
func SortVersionsDescending(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		v1 := NormalizeVersion(versions[i])
		v2 := NormalizeVersion(versions[j])
		if semver.IsValid(v1) && semver.IsValid(v2) {
			return semver.Compare(v1, v2) > 0
		}
		return versions[i] > versions[j]
	})
}

// NormalizeVersion ensures a version string has a "v" prefix for semver compatibility.
func NormalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}
