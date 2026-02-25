package infrastructure

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// DiscovererFactory is a constructor that creates a RepositoryDiscoverer given an auth token.
type DiscovererFactory func(token string) globalEntities.RepositoryDiscoverer
