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

	// SSHCloneURL returns an SSH clone URL for the repository.
	//
	// sshAlias is the suffix that will be appended to the provider's default SSH
	// hostname to form an ssh_config Host entry. For example, passing "mine"
	// allows providers to construct a host like "github.com-mine", which can
	// then map to an entry in ~/.ssh/config.
	//
	// Implementations SHOULD treat an empty sshAlias as "no alias": in that
	// case they MUST generate a URL that uses the default SSH hostname
	// (for example, "git@github.com:org/repo.git") without appending any
	// suffix (and without a trailing dash).
	SSHCloneURL(repo Repository, sshAlias string) string

	// DiscoverRepositories lists all repositories in an organization or group.
	DiscoverRepositories(ctx context.Context, org string) ([]Repository, error)

	// CreatePullRequest creates a pull/merge request on the hosting service.
	CreatePullRequest(
		ctx context.Context, repo Repository, input PullRequestInput,
	) (*PullRequest, error)

	// PullRequestExists checks if an open pull request already exists for the given source branch.
	PullRequestExists(ctx context.Context, repo Repository, sourceBranch string) (bool, error)
}
