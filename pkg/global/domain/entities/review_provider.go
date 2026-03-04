package entities

import "context"

// ReviewProvider extends ForgeProvider with pull request review operations.
// This is used by tools that review pull requests (e.g. code-guru).
type ReviewProvider interface {
	ForgeProvider

	// ListOpenPullRequests returns all open/active pull requests for a repository.
	ListOpenPullRequests(
		ctx context.Context, repo Repository,
	) ([]PullRequestDetail, error)

	// GetPullRequestDiff returns the full unified diff for a specific pull request.
	GetPullRequestDiff(
		ctx context.Context, repo Repository, prID int,
	) (string, error)

	// GetPullRequestFiles returns the list of changed files in a pull request.
	GetPullRequestFiles(
		ctx context.Context, repo Repository, prID int,
	) ([]PullRequestFile, error)

	// PostPullRequestComment posts a general comment on a pull request.
	PostPullRequestComment(
		ctx context.Context, repo Repository, prID int, body string,
	) error

	// PostPullRequestThreadComment posts an inline/thread comment on a specific file and line.
	PostPullRequestThreadComment(
		ctx context.Context, repo Repository, prID int,
		filePath string, line int, body string,
	) error
}
