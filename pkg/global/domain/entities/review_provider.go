package entities

import "context"

// CommentOption configures behavior of PostPullRequestComment and
// PostPullRequestThreadComment. Use the With* helpers (e.g. WithThreadStatus)
// rather than constructing the option type directly so the underlying option
// struct can grow new fields without breaking callers.
type CommentOption func(*commentOptions)

// commentOptions captures the resolved option values applied by CommentOption
// helpers. The struct is intentionally unexported so providers reach into it
// only through ResolveCommentOptions.
type commentOptions struct {
	status string
}

// DefaultCommentStatus is the thread status applied by PostPullRequestComment
// and PostPullRequestThreadComment when the caller does not pass
// WithThreadStatus. It matches the historical behavior of both methods, which
// hard-coded `"active"` in the request payload before this option existed.
const DefaultCommentStatus = "active"

// WithThreadStatus sets the initial thread status sent to the underlying
// provider when the comment is created. Accepted values are provider-specific;
// Azure DevOps recognises `"active"`, `"fixed"`, `"closed"`, `"wontFix"`,
// `"byDesign"`, and `"pending"`. Defaults to DefaultCommentStatus when not
// provided. Providers that do not expose a thread-status concept (e.g. GitHub
// REST review comments) silently ignore the value.
func WithThreadStatus(status string) CommentOption {
	return func(o *commentOptions) {
		o.status = status
	}
}

// ResolveCommentOptions applies the given CommentOption helpers in order and
// returns the resolved status string. Provider implementations should call
// this at the top of PostPullRequestComment and PostPullRequestThreadComment
// to pick up any caller-supplied overrides while preserving the documented
// default.
func ResolveCommentOptions(opts ...CommentOption) string {
	resolved := commentOptions{status: DefaultCommentStatus}
	for _, opt := range opts {
		if opt != nil {
			opt(&resolved)
		}
	}
	return resolved.status
}

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

	// PostPullRequestComment posts a general comment on a pull request. Optional
	// CommentOption helpers tune the resulting thread (e.g. WithThreadStatus to
	// post the comment as `"fixed"`/`"closed"` instead of the default
	// `"active"` so reviewers don't have to dismiss informational annotations).
	PostPullRequestComment(
		ctx context.Context, repo Repository, prID int, body string,
		opts ...CommentOption,
	) error

	// PostPullRequestThreadComment posts an inline/thread comment on a specific file and line.
	// Returns a provider-specific identifier for the newly created comment / thread / review.
	// On Azure DevOps the value is the thread ID returned by the threads API and is suitable
	// for passing to UpdatePullRequestThreadStatus. On GitHub the value is the pull-request
	// review ID returned by the reviews API; GitHub does not expose REST thread-status
	// updates so the ID is informational and UpdatePullRequestThreadStatus on GitHub returns
	// an unsupported error regardless. Callers that need the marker-thread auto-close
	// pattern should check the provider before relying on the ID — pinned per Copilot
	// review on PR #86 thread `PRRT_kwDORQWb3M5-6QBC`.
	//
	// Optional CommentOption helpers tune the resulting thread (e.g.
	// WithThreadStatus to post informational annotations as `"fixed"`/`"closed"`
	// instead of the default `"active"`). Providers that do not expose a
	// thread-status concept silently ignore the option.
	PostPullRequestThreadComment(
		ctx context.Context, repo Repository, prID int,
		filePath string, line int, body string,
		opts ...CommentOption,
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
