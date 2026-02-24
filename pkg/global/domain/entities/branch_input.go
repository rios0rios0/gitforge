package entities

// BranchInput contains the data needed to create a branch with file changes.
type BranchInput struct {
	BranchName    string
	BaseBranch    string
	Changes       []FileChange
	CommitMessage string
}
