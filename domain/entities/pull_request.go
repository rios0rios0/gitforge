package entities

// BranchInput contains the data needed to create a branch with file changes.
type BranchInput struct {
	BranchName    string
	BaseBranch    string
	Changes       []FileChange
	CommitMessage string
}

// PullRequestInput contains the data needed to create a pull request.
type PullRequestInput struct {
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
	AutoComplete bool
}

// PullRequest represents a pull/merge request returned by a provider.
type PullRequest struct {
	ID     int
	Title  string
	URL    string
	Status string
}

// PullRequestDetail extends PullRequest with review-relevant metadata.
type PullRequestDetail struct {
	PullRequest

	SourceBranch string
	TargetBranch string
	Author       string
}

// PullRequestFile represents a single file changed in a pull request.
type PullRequestFile struct {
	Path      string
	OldPath   string
	Status    string // "added", "modified", "deleted", "renamed"
	Additions int
	Deletions int
	Patch     string // unified diff patch for this file
}
