package entities

// PullRequestDetail extends PullRequest with review-relevant metadata.
type PullRequestDetail struct {
	PullRequest

	SourceBranch string
	TargetBranch string
	Author       string
}
