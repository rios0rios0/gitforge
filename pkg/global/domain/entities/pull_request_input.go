package entities

// PullRequestInput contains the data needed to create a pull request.
type PullRequestInput struct {
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
	AutoComplete bool
}
