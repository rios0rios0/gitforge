package builders

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	testkit "github.com/rios0rios0/testkit/pkg/test"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/test/doubles"
)

// ForgeProviderStubBuilder builds ForgeProviderStub instances using the builder pattern.
type ForgeProviderStubBuilder struct {
	*testkit.BaseBuilder

	name        string
	matchURL    string
	token       string
	serviceType globalEntities.ServiceType
	authMethods []transport.AuthMethod
}

// NewForgeProviderStubBuilder creates a new builder with default values.
func NewForgeProviderStubBuilder() *ForgeProviderStubBuilder {
	return &ForgeProviderStubBuilder{BaseBuilder: testkit.NewBaseBuilder()}
}

func (b *ForgeProviderStubBuilder) WithName(name string) *ForgeProviderStubBuilder {
	b.name = name
	return b
}

func (b *ForgeProviderStubBuilder) WithMatchURL(matchURL string) *ForgeProviderStubBuilder {
	b.matchURL = matchURL
	return b
}

func (b *ForgeProviderStubBuilder) WithToken(token string) *ForgeProviderStubBuilder {
	b.token = token
	return b
}

func (b *ForgeProviderStubBuilder) WithServiceType(
	serviceType globalEntities.ServiceType,
) *ForgeProviderStubBuilder {
	b.serviceType = serviceType
	return b
}

func (b *ForgeProviderStubBuilder) WithAuthMethods(
	authMethods []transport.AuthMethod,
) *ForgeProviderStubBuilder {
	b.authMethods = authMethods
	return b
}

func (b *ForgeProviderStubBuilder) Build() any {
	return &doubles.ForgeProviderStub{
		NameValue:        b.name,
		MatchURLValue:    b.matchURL,
		TokenValue:       b.token,
		ServiceTypeValue: b.serviceType,
		AuthMethodsValue: b.authMethods,
	}
}
