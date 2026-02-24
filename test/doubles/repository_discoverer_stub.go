package doubles

import (
	"context"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
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

// RepositoryDiscovererStubBuilder builds RepositoryDiscovererStub instances using the builder pattern.
type RepositoryDiscovererStubBuilder struct {
	*testkit.BaseBuilder

	name string
}

// NewRepositoryDiscovererStubBuilder creates a new builder with default values.
func NewRepositoryDiscovererStubBuilder() *RepositoryDiscovererStubBuilder {
	return &RepositoryDiscovererStubBuilder{BaseBuilder: testkit.NewBaseBuilder()}
}

func (b *RepositoryDiscovererStubBuilder) WithName(
	name string,
) *RepositoryDiscovererStubBuilder {
	b.name = name
	return b
}

func (b *RepositoryDiscovererStubBuilder) Build() any {
	return &RepositoryDiscovererStub{NameValue: b.name}
}
