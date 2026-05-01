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
	// Returns the newly created thread ID, which can be used later with
	// UpdatePullRequestThreadStatus to mark the thread as fixed/closed.
	PostPullRequestThreadComment(
		ctx context.Context, repo Repository, prID int,
		filePath string, line int, body string,
	) (int, error)

	// UpdatePullRequestThreadStatus updates the status of an existing pull request thread
	// (e.g. "fixed", "closed", "active"). The exact set of valid status strings is
	// provider-specific. Providers that do not support thread status updates may return
	// an error indicating the operation is unsupported.
	UpdatePullRequestThreadStatus(
		ctx context.Context, repo Repository, prID, threadID int, status string,
	) error

	// GetPullRequestStatus returns the current status of a pull request as a string
	// (e.g. "active", "completed", "abandoned", "merged", "closed"). The exact set of
	// possible values is provider-specific.
	GetPullRequestStatus(
		ctx context.Context, repo Repository, prID int,
	) (string, error)

	// GetPullRequestCheckStatus returns whether all CI checks/statuses have passed for a pull request.
	GetPullRequestCheckStatus(
		ctx context.Context, repo Repository, prID int,
	) (bool, error)

	// MergePullRequest merges a pull request using the specified strategy (e.g. "merge", "squash", "rebase").
	MergePullRequest(
		ctx context.Context, repo Repository, prID int, strategy string,
	) error
}
