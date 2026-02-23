package registry_test

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
	"github.com/rios0rios0/gitforge/domain/repositories"
	"github.com/rios0rios0/gitforge/infrastructure/registry"
)

// stubProvider is a minimal ForgeProvider + LocalGitAuthProvider for testing.
type stubProvider struct {
	name        string
	matchURL    string
	serviceType entities.ServiceType
	token       string
}

func (s *stubProvider) Name() string      { return s.name }
func (s *stubProvider) AuthToken() string { return s.token }

func (s *stubProvider) MatchesURL(rawURL string) bool {
	return len(s.matchURL) > 0 && rawURL == s.matchURL
}

func (s *stubProvider) CloneURL(_ entities.Repository) string { return "" }

func (s *stubProvider) DiscoverRepositories(
	_ context.Context, _ string,
) ([]entities.Repository, error) {
	return nil, nil
}

func (s *stubProvider) CreatePullRequest(
	_ context.Context, _ entities.Repository, _ entities.PullRequestInput,
) (*entities.PullRequest, error) {
	return nil, nil //nolint:nilnil // test stub, method is not exercised
}

func (s *stubProvider) PullRequestExists(
	_ context.Context, _ entities.Repository, _ string,
) (bool, error) {
	return false, nil
}

func (s *stubProvider) GetServiceType() entities.ServiceType { return s.serviceType }
func (s *stubProvider) PrepareCloneURL(url string) string    { return url }
func (s *stubProvider) ConfigureTransport()                  {}

func (s *stubProvider) GetAuthMethods(_ string) []transport.AuthMethod {
	return nil
}

// stubDiscoverer is a minimal RepositoryDiscoverer for testing.
type stubDiscoverer struct {
	name string
}

func (d *stubDiscoverer) Name() string { return d.name }

func (d *stubDiscoverer) DiscoverRepositories(
	_ context.Context, _ string,
) ([]entities.Repository, error) {
	return nil, nil
}

func TestNewProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("should create empty registry", func(t *testing.T) {
		t.Parallel()

		// given / when
		reg := registry.NewProviderRegistry()

		// then
		require.NotNil(t, reg)
		assert.Empty(t, reg.Names())
	})
}

func TestProviderRegistryGet(t *testing.T) {
	t.Parallel()

	t.Run("should return provider when factory is registered", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		reg.RegisterFactory("test", func(token string) repositories.ForgeProvider {
			return &stubProvider{name: "test", token: token}
		})

		// when
		provider, err := reg.Get("test", "my-token")

		// then
		require.NoError(t, err)
		assert.Equal(t, "test", provider.Name())
		assert.Equal(t, "my-token", provider.AuthToken())
	})

	t.Run("should return error when factory is not registered", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()

		// when
		_, err := reg.Get("unknown", "token")

		// then
		require.Error(t, err)
	})
}

func TestProviderRegistryGetDiscoverer(t *testing.T) {
	t.Parallel()

	t.Run("should return discoverer when factory is registered", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		reg.RegisterDiscoverer("test", func(_ string) entities.RepositoryDiscoverer {
			return &stubDiscoverer{name: "test"}
		})

		// when
		discoverer, err := reg.GetDiscoverer("test", "token")

		// then
		require.NoError(t, err)
		assert.Equal(t, "test", discoverer.Name())
	})

	t.Run("should return error when discoverer is not registered", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()

		// when
		_, err := reg.GetDiscoverer("unknown", "token")

		// then
		require.Error(t, err)
	})
}

func TestProviderRegistryGetAdapterByURL(t *testing.T) {
	t.Parallel()

	t.Run("should return adapter when URL matches", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		adapter := &stubProvider{name: "github", matchURL: "https://github.com/org/repo"}
		reg.RegisterAdapter(adapter)

		// when
		result := reg.GetAdapterByURL("https://github.com/org/repo")

		// then
		require.NotNil(t, result)
		assert.Equal(t, "github", result.Name())
	})

	t.Run("should return nil when no adapter matches URL", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		adapter := &stubProvider{name: "github", matchURL: "https://github.com/org/repo"}
		reg.RegisterAdapter(adapter)

		// when
		result := reg.GetAdapterByURL("https://gitlab.com/org/repo")

		// then
		assert.Nil(t, result)
	})
}

func TestProviderRegistryGetAdapterByServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return adapter when service type matches", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		adapter := &stubProvider{name: "github", serviceType: entities.GITHUB}
		reg.RegisterAdapter(adapter)

		// when
		result := reg.GetAdapterByServiceType(entities.GITHUB)

		// then
		require.NotNil(t, result)
		assert.Equal(t, entities.GITHUB, result.GetServiceType())
	})

	t.Run("should return nil when service type does not match", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		adapter := &stubProvider{name: "github", serviceType: entities.GITHUB}
		reg.RegisterAdapter(adapter)

		// when
		result := reg.GetAdapterByServiceType(entities.GITLAB)

		// then
		assert.Nil(t, result)
	})

	t.Run("should return nil when adapter does not implement LocalGitAuthProvider", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		// Register a factory-only provider (no adapter registered)

		// when
		result := reg.GetAdapterByServiceType(entities.GITHUB)

		// then
		assert.Nil(t, result)
	})
}

func TestProviderRegistryNames(t *testing.T) {
	t.Parallel()

	t.Run("should return all registered factory names", func(t *testing.T) {
		t.Parallel()

		// given
		reg := registry.NewProviderRegistry()
		reg.RegisterFactory("github", func(_ string) repositories.ForgeProvider {
			return &stubProvider{name: "github"}
		})
		reg.RegisterFactory("gitlab", func(_ string) repositories.ForgeProvider {
			return &stubProvider{name: "gitlab"}
		})

		// when
		names := reg.Names()

		// then
		assert.Len(t, names, 2)
		assert.ElementsMatch(t, []string{"github", "gitlab"}, names)
	})
}
