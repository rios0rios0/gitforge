package doubles

import (
	"context"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// RepositoryDiscovererStub implements RepositoryDiscoverer for testing.
type RepositoryDiscovererStub struct {
	NameValue string
}

func (s *RepositoryDiscovererStub) Name() string { return s.NameValue }

func (s *RepositoryDiscovererStub) DiscoverRepositories(
	_ context.Context, _ string,
) ([]globalEntities.Repository, error) {
	return nil, nil
}
