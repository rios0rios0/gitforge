package entities

// File represents a file entry within a repository.
type File struct {
	Path     string
	ObjectID string
	IsDir    bool
}

// FileChange represents a file modification to be included in a commit.
type FileChange struct {
	Path       string
	Content    string
	ChangeType string // "add", "edit", "delete"
}
