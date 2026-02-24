package entities

import "context"

// ForgeProvider is the core interface for any Git hosting provider.
// It provides common operations: URL matching, discovery, PR management, and authentication.
type ForgeProvider interface {
	// Name returns the provider identifier (e.g. "github", "gitlab", "azuredevops").
	Name() string

	// MatchesURL returns true if the given remote URL belongs to this provider.
	MatchesURL(url string) bool

	// AuthToken returns the authentication token configured for this provider.
	AuthToken() string

	// CloneURL returns an HTTPS clone URL for the repository, potentially with
	// embedded credentials for authenticated access.
	CloneURL(repo Repository) string

	// DiscoverRepositories lists all repositories in an organization or group.
	DiscoverRepositories(ctx context.Context, org string) ([]Repository, error)

	// CreatePullRequest creates a pull/merge request on the hosting service.
	CreatePullRequest(
		ctx context.Context, repo Repository, input PullRequestInput,
	) (*PullRequest, error)

	// PullRequestExists checks if an open pull request already exists for the given source branch.
	PullRequestExists(ctx context.Context, repo Repository, sourceBranch string) (bool, error)
}
