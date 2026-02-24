package doubles

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing/transport"
	testkit "github.com/rios0rios0/testkit/pkg/test"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// ForgeProviderStub implements ForgeProvider + LocalGitAuthProvider for testing.
type ForgeProviderStub struct {
	NameValue        string
	MatchURLValue    string
	TokenValue       string
	ServiceTypeValue globalEntities.ServiceType
	AuthMethodsValue []transport.AuthMethod
}

func (s *ForgeProviderStub) Name() string      { return s.NameValue }
func (s *ForgeProviderStub) AuthToken() string { return s.TokenValue }
func (s *ForgeProviderStub) MatchesURL(rawURL string) bool {
	return len(s.MatchURLValue) > 0 && rawURL == s.MatchURLValue
}
func (s *ForgeProviderStub) CloneURL(_ globalEntities.Repository) string { return "" }
func (s *ForgeProviderStub) DiscoverRepositories(
	_ context.Context, _ string,
) ([]globalEntities.Repository, error) {
	return nil, nil
}
func (s *ForgeProviderStub) CreatePullRequest(
	_ context.Context, _ globalEntities.Repository, _ globalEntities.PullRequestInput,
) (*globalEntities.PullRequest, error) {
	return nil, nil //nolint:nilnil // test stub, method is not exercised
}
func (s *ForgeProviderStub) PullRequestExists(
	_ context.Context, _ globalEntities.Repository, _ string,
) (bool, error) {
	return false, nil
}
func (s *ForgeProviderStub) GetServiceType() globalEntities.ServiceType {
	return s.ServiceTypeValue
}
func (s *ForgeProviderStub) PrepareCloneURL(url string) string { return url }
func (s *ForgeProviderStub) ConfigureTransport()               {}
func (s *ForgeProviderStub) GetAuthMethods(_ string) []transport.AuthMethod {
	return s.AuthMethodsValue
}

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
	return &ForgeProviderStub{
		NameValue:        b.name,
		MatchURLValue:    b.matchURL,
		TokenValue:       b.token,
		ServiceTypeValue: b.serviceType,
		AuthMethodsValue: b.authMethods,
	}
}
