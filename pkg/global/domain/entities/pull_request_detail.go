package entities

// PullRequestDetail extends PullRequest with review-relevant metadata.
type PullRequestDetail struct {
	PullRequest

	SourceBranch string
	TargetBranch string
	Author       string

	// IsDraft reports whether the PR is in draft / "work in progress" state.
	// On GitHub this is the `draft` boolean; on Azure DevOps it is the
	// `isDraft` boolean. Providers populate this on every PullRequestDetail
	// they emit so consumers can decide whether to skip drafts. The
	// ListOpenPullRequests methods intentionally do NOT filter drafts —
	// the policy lives in the consumer.
	IsDraft bool
}
