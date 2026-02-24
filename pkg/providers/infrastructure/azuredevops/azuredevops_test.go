package azuredevops_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/pkg/providers/infrastructure/azuredevops"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	t.Run("should create provider with given token", func(t *testing.T) {
		t.Parallel()

		// given
		token := "ado-pat-test123"

		// when
		provider := azuredevops.NewProvider(token)

		// then
		require.NotNil(t, provider)
		assert.Equal(t, token, provider.AuthToken())
	})
}

func TestProviderName(t *testing.T) {
	t.Parallel()

	t.Run("should return azuredevops as provider name", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")

		// when
		name := provider.Name()

		// then
		assert.Equal(t, "azuredevops", name)
	})
}

func TestProviderMatchesURL(t *testing.T) {
	t.Parallel()

	t.Run("should match dev.azure.com URLs", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")

		// when
		result := provider.MatchesURL("https://dev.azure.com/org/project/_git/repo")

		// then
		assert.True(t, result)
	})

	t.Run("should not match non-azure URLs", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")

		// when
		result := provider.MatchesURL("https://github.com/org/repo.git")

		// then
		assert.False(t, result)
	})
}

func TestProviderCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should embed token in clone URL when remote URL exists", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("$test-token")
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			Name:         "my-repo",
			RemoteURL:    "https://dev.azure.com/my-org/my-project/_git/my-repo",
		}

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://pat:$test-token@dev.azure.com/my-org/my-project/_git/my-repo", result)
	})

	t.Run("should construct clone URL when remote URL is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("$test-token")
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			Name:         "my-repo",
		}

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://pat:$test-token@dev.azure.com/my-org/my-project/_git/my-repo", result)
	})
}

func TestProviderGetServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return AZUREDEVOPS service type", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		result := lgap.GetServiceType()

		// then
		assert.Equal(t, globalEntities.AZUREDEVOPS, result)
	})
}

func TestProviderPrepareCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should strip username from URL", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")
		rawURL := "https://user@dev.azure.com/org/project/_git/repo"

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		result := lgap.PrepareCloneURL(rawURL)

		// then
		assert.Equal(t, "https://dev.azure.com/org/project/_git/repo", result)
	})

	t.Run("should return URL unchanged when no username present", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")
		rawURL := "https://dev.azure.com/org/project/_git/repo"

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		result := lgap.PrepareCloneURL(rawURL)

		// then
		assert.Equal(t, rawURL, result)
	})
}

func TestProviderGetAuthMethods(t *testing.T) {
	t.Parallel()

	t.Run("should return auth methods when token is set", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("$test-token")

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("user")

		// then
		assert.Len(t, methods, 1)
	})

	t.Run("should return empty auth methods when token is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("")

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("user")

		// then
		assert.Empty(t, methods)
	})
}
