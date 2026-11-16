package entities

import "context"

// OpenPullRequestFinder is implemented by providers that can return the open
// pull request of a source branch, not only report whether one exists.
//
// It is an optional extension of [ForgeProvider] rather than a method on it, so
// adding it breaks no existing implementation; consumers type-assert to it, the
// same way they do for [ReviewProvider].
type OpenPullRequestFinder interface {
	// FindOpenPullRequest returns the open (on Azure DevOps, active) pull request
	// whose source branch is sourceBranch. The boolean reports whether one
	// exists; when it is false, the returned pull request is the zero value.
	FindOpenPullRequest(ctx context.Context, repo Repository, sourceBranch string) (PullRequest, bool, error)
}
