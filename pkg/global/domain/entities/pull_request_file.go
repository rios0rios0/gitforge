package entities

// PullRequestFile represents a single file changed in a pull request.
type PullRequestFile struct {
	Path      string
	OldPath   string
	Status    string // "added", "modified", "deleted", "renamed"
	Additions int
	Deletions int
	Patch     string // unified diff patch for this file
}
