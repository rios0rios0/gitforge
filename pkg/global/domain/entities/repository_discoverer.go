package entities

import "context"

// RepositoryDiscoverer can list repositories from a Git hosting provider.
type RepositoryDiscoverer interface {
	// Name returns the provider identifier (e.g. "github").
	Name() string
	// DiscoverRepositories lists all repositories in an organization or group.
	DiscoverRepositories(ctx context.Context, org string) ([]Repository, error)
}
