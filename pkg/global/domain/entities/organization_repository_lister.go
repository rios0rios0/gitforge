package entities

import "context"

// OrganizationRepositoryLister is implemented by providers that can list every
// repository of an organization in one strict call.
//
// Unlike [ForgeProvider.DiscoverRepositories], which skips a part of the
// organization it cannot read and keeps going, a lister either returns the
// whole organization or an error, so a caller that must not mistake a partial
// listing for a complete one can rely on the result. Consumers type-assert to
// it, the same way they do for [ReviewProvider].
type OrganizationRepositoryLister interface {
	// ListOrganizationRepositories returns every repository of org, with its
	// project, fork, archived, and disabled state, or an error when any part of
	// the listing fails.
	ListOrganizationRepositories(ctx context.Context, org string) ([]Repository, error)
}
