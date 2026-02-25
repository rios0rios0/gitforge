package builders

import (
	"github.com/rios0rios0/gitforge/test/doubles"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

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
	return &doubles.RepositoryDiscovererStub{NameValue: b.name}
}
