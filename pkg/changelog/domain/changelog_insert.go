package domain

import "strings"

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
