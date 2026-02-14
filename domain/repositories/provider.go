package repositories

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/rios0rios0/gitforge/domain/entities"
)

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
	CloneURL(repo entities.Repository) string

	// DiscoverRepositories lists all repositories in an organization or group.
	DiscoverRepositories(ctx context.Context, org string) ([]entities.Repository, error)

	// CreatePullRequest creates a pull/merge request on the hosting service.
	CreatePullRequest(
		ctx context.Context, repo entities.Repository, input entities.PullRequestInput,
	) (*entities.PullRequest, error)

	// PullRequestExists checks if an open pull request already exists for the given source branch.
	PullRequestExists(ctx context.Context, repo entities.Repository, sourceBranch string) (bool, error)
}

// FileAccessProvider extends ForgeProvider with API-based file operations.
type FileAccessProvider interface {
	ForgeProvider

	// GetFileContent reads the content of a file from a repository's default branch.
	GetFileContent(ctx context.Context, repo entities.Repository, path string) (string, error)

	// ListFiles returns the list of files in a repository, optionally filtered by a path pattern.
	ListFiles(ctx context.Context, repo entities.Repository, pattern string) ([]entities.File, error)

	// GetTags returns all tags for a repository, sorted by semantic version descending.
	GetTags(ctx context.Context, repo entities.Repository) ([]string, error)

	// HasFile checks whether a file exists at the given path in a repository.
	HasFile(ctx context.Context, repo entities.Repository, path string) bool

	// CreateBranchWithChanges creates a new branch with one or more file changes
	// committed on top of the base branch.
	CreateBranchWithChanges(ctx context.Context, repo entities.Repository, input entities.BranchInput) error
}

// LocalGitAuthProvider extends ForgeProvider with local git authentication.
// This is used by tools that perform local git operations (clone, push) via go-git.
type LocalGitAuthProvider interface {
	ForgeProvider

	// GetServiceType returns the service type identifier for this provider.
	GetServiceType() entities.ServiceType

	// PrepareCloneURL processes the URL before cloning (e.g., stripping embedded credentials).
	PrepareCloneURL(url string) string

	// ConfigureTransport configures any transport-level settings required by this service.
	ConfigureTransport()

	// GetAuthMethods returns the authentication methods for local git operations.
	GetAuthMethods(username string) []transport.AuthMethod
}
