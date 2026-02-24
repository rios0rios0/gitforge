package entities

import "context"

// FileAccessProvider extends ForgeProvider with API-based file operations.
type FileAccessProvider interface {
	ForgeProvider

	// GetFileContent reads the content of a file from a repository's default branch.
	GetFileContent(ctx context.Context, repo Repository, path string) (string, error)

	// ListFiles returns the list of files in a repository, optionally filtered by a path pattern.
	ListFiles(ctx context.Context, repo Repository, pattern string) ([]File, error)

	// GetTags returns all tags for a repository, sorted by semantic version descending.
	GetTags(ctx context.Context, repo Repository) ([]string, error)

	// HasFile checks whether a file exists at the given path in a repository.
	HasFile(ctx context.Context, repo Repository, path string) bool

	// CreateBranchWithChanges creates a new branch with one or more file changes
	// committed on top of the base branch.
	CreateBranchWithChanges(ctx context.Context, repo Repository, input BranchInput) error
}
