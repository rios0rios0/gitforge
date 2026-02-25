package doubles

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing/transport"

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
