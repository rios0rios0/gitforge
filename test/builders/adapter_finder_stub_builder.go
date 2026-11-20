package builders

import (
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/test/doubles"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

// AdapterFinderStubBuilder builds AdapterFinderStub instances using the builder pattern.
type AdapterFinderStubBuilder struct {
	*testkit.BaseBuilder

	adapterByServiceType globalEntities.LocalGitAuthProvider
	adapterByURL         globalEntities.LocalGitAuthProvider
	adaptersByURL        map[string]globalEntities.LocalGitAuthProvider
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

// WithAdapterForURL makes GetAdapterByURL answer adapter for url alone, ahead of the adapter
// WithAdapterByURL sets for every URL.
func (b *AdapterFinderStubBuilder) WithAdapterForURL(
	url string,
	adapter globalEntities.LocalGitAuthProvider,
) *AdapterFinderStubBuilder {
	if b.adaptersByURL == nil {
		b.adaptersByURL = make(map[string]globalEntities.LocalGitAuthProvider)
	}
	b.adaptersByURL[url] = adapter
	return b
}

func (b *AdapterFinderStubBuilder) Build() any {
	return &doubles.AdapterFinderStub{
		AdapterByServiceTypeValue: b.adapterByServiceType,
		AdapterByURLValue:         b.adapterByURL,
		AdaptersByURL:             b.adaptersByURL,
	}
}
