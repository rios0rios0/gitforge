package builders

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/test/doubles"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

// AdapterFinderStubBuilder builds AdapterFinderStub instances using the builder pattern.
type AdapterFinderStubBuilder struct {
	*testkit.BaseBuilder

	adapterByServiceType globalEntities.LocalGitAuthProvider
	adapterByURL         globalEntities.LocalGitAuthProvider
}

// NewAdapterFinderStubBuilder creates a new builder with default values.
func NewAdapterFinderStubBuilder() *AdapterFinderStubBuilder {
	return &AdapterFinderStubBuilder{BaseBuilder: testkit.NewBaseBuilder()}
}

func (b *AdapterFinderStubBuilder) WithAdapterByServiceType(
	adapter globalEntities.LocalGitAuthProvider,
) *AdapterFinderStubBuilder {
	b.adapterByServiceType = adapter
	return b
}

func (b *AdapterFinderStubBuilder) WithAdapterByURL(
	adapter globalEntities.LocalGitAuthProvider,
) *AdapterFinderStubBuilder {
	b.adapterByURL = adapter
	return b
}

func (b *AdapterFinderStubBuilder) Build() any {
	return &doubles.AdapterFinderStub{
		AdapterByServiceTypeValue: b.adapterByServiceType,
		AdapterByURLValue:         b.adapterByURL,
	}
}
