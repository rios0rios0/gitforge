package entities

import "strings"

const (
	unreleasedHeading = "## [Unreleased]"
	changedSubheading = "### Changed"
	h2Prefix          = "## ["
	bulletPrefix      = "- "
	// asteriskBulletPrefix is the other marker a hand-written changelog may
	// use for the very same list.
	asteriskBulletPrefix = "* "
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

// findLastBullet returns the index of the last line of the last bullet under
// the Changed heading, or the heading itself when it holds no bullet.
//
// A bullet's last line is not always the one that starts it: a wrapped entry
// continues on the lines below, indented or not. Stopping at the first line
// that is not a bullet put new entries between a bullet and the rest of its own
// sentence, and the continuation line then read as the tail of the new entry.
// A blank line ends a bullet, a heading ends the list, and a line after a blank
// one still belongs to the bullet above only when it is indented.
func findLastBullet(lines []string, changedIdx, endIdx int) int {
	insertAfter := changedIdx
	inBullet := false
	for i := changedIdx + 1; i < endIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case trimmed == "":
			inBullet = false
		case strings.HasPrefix(trimmed, "#"):
			return insertAfter
		case isBulletLine(trimmed):
			insertAfter, inBullet = i, true
		case inBullet, insertAfter > changedIdx && isIndented(lines[i]):
			insertAfter, inBullet = i, true
		default:
			return insertAfter
		}
	}
	return insertAfter
}

// isBulletLine reports whether an already-trimmed line opens a list item, with
// either of the markers Keep a Changelog files are written with.
func isBulletLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, bulletPrefix) || strings.HasPrefix(trimmed, asteriskBulletPrefix)
}

// isIndented reports whether a line starts with whitespace.
func isIndented(line string) bool {
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

func insertLinesAt(lines []string, at int, extra []string) []string {
	result := make([]string, 0, len(lines)+len(extra))
	result = append(result, lines[:at]...)
	result = append(result, extra...)
	result = append(result, lines[at:]...)
	return result
}
