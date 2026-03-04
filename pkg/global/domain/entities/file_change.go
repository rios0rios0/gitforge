package entities

// FileChange represents a file modification to be included in a commit.
type FileChange struct {
	Path       string
	Content    string
	ChangeType string // "add", "edit", "delete"
}
