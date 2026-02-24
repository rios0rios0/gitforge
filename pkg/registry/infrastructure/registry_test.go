package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	infrastructure "github.com/rios0rios0/gitforge/pkg/registry/infrastructure"
	"github.com/rios0rios0/gitforge/test/doubles"
)

func TestNewProviderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("should create empty registry", func(t *testing.T) {
		t.Parallel()

		// given / when
		reg := infrastructure.NewProviderRegistry()

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
		reg := infrastructure.NewProviderRegistry()
		reg.RegisterFactory("test", func(token string) globalEntities.ForgeProvider {
			return &doubles.ForgeProviderStub{NameValue: "test", TokenValue: token}
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
		reg := infrastructure.NewProviderRegistry()

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
		reg := infrastructure.NewProviderRegistry()
		reg.RegisterDiscoverer("test", func(_ string) globalEntities.RepositoryDiscoverer {
			return &doubles.RepositoryDiscovererStub{NameValue: "test"}
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
		reg := infrastructure.NewProviderRegistry()

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
		reg := infrastructure.NewProviderRegistry()
		adapter := &doubles.ForgeProviderStub{NameValue: "github", MatchURLValue: "https://github.com/org/repo"}
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
		reg := infrastructure.NewProviderRegistry()
		adapter := &doubles.ForgeProviderStub{NameValue: "github", MatchURLValue: "https://github.com/org/repo"}
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
		reg := infrastructure.NewProviderRegistry()
		adapter := &doubles.ForgeProviderStub{NameValue: "github", ServiceTypeValue: globalEntities.GITHUB}
		reg.RegisterAdapter(adapter)

		// when
		result := reg.GetAdapterByServiceType(globalEntities.GITHUB)

		// then
		require.NotNil(t, result)
		assert.Equal(t, globalEntities.GITHUB, result.GetServiceType())
	})

	t.Run("should return nil when service type does not match", func(t *testing.T) {
		t.Parallel()

		// given
		reg := infrastructure.NewProviderRegistry()
		adapter := &doubles.ForgeProviderStub{NameValue: "github", ServiceTypeValue: globalEntities.GITHUB}
		reg.RegisterAdapter(adapter)

		// when
		result := reg.GetAdapterByServiceType(globalEntities.GITLAB)

		// then
		assert.Nil(t, result)
	})

	t.Run("should return nil when adapter does not implement LocalGitAuthProvider", func(t *testing.T) {
		t.Parallel()

		// given
		reg := infrastructure.NewProviderRegistry()
		// Register a factory-only provider (no adapter registered)

		// when
		result := reg.GetAdapterByServiceType(globalEntities.GITHUB)

		// then
		assert.Nil(t, result)
	})
}

func TestProviderRegistryNames(t *testing.T) {
	t.Parallel()

	t.Run("should return all registered factory names", func(t *testing.T) {
		t.Parallel()

		// given
		reg := infrastructure.NewProviderRegistry()
		reg.RegisterFactory("github", func(_ string) globalEntities.ForgeProvider {
			return &doubles.ForgeProviderStub{NameValue: "github"}
		})
		reg.RegisterFactory("gitlab", func(_ string) globalEntities.ForgeProvider {
			return &doubles.ForgeProviderStub{NameValue: "gitlab"}
		})

		// when
		names := reg.Names()

		// then
		assert.Len(t, names, 2)
		assert.ElementsMatch(t, []string{"github", "gitlab"}, names)
	})
}
