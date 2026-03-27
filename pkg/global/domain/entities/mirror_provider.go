package entities

import "context"

// MirrorInput contains the data needed to create a mirror repository on a target provider.
type MirrorInput struct {
	CloneAddr   string // source URL (e.g., https://github.com/org/repo.git)
	RepoName    string
	RepoOwner   string
	Private     bool
	Description string
	Mirror      bool   // true = create as pull mirror
	Service     string // source service type (e.g., "github", "gitlab")
}

// MirrorProvider extends ForgeProvider with repository migration/mirror capabilities.
type MirrorProvider interface {
	ForgeProvider

	// MigrateRepository creates a repository on the target provider by migrating
	// from a source URL. When Mirror is true, the target provider will periodically
	// pull updates from the source.
	MigrateRepository(ctx context.Context, input MirrorInput) error
}
