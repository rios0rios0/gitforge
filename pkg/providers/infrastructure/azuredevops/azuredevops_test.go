package azuredevops_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/pkg/providers/infrastructure/azuredevops"
	"github.com/rios0rios0/gitforge/v4/test/builders"
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

	for _, scenario := range []struct{ name, rawURL string }{
		{"an HTTPS remote on dev.azure.com", "https://dev.azure.com/org/project/_git/repo"},
		{"an HTTPS remote with the organization as its user", "https://org@dev.azure.com/org/project/_git/repo"},
		{"an HTTPS remote on an organization's visualstudio.com host", "https://org.visualstudio.com/project/_git/repo"},
		{"an HTTPS remote in a visualstudio.com collection", "https://org.visualstudio.com/DefaultCollection/project/_git/repo"},
		{"a scp-like SSH remote", "git@ssh.dev.azure.com:v3/org/project/repo"},
		{"an SSH host alias", "git@dev.azure.com-work:v3/org/project/repo"},
		{"an ssh:// remote", "ssh://git@ssh.dev.azure.com/v3/org/project/repo"},
	} {
		t.Run("should match when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			provider := azuredevops.NewProvider("token")

			// when
			result := provider.MatchesURL(scenario.rawURL)

			// then
			assert.True(t, result)
		})
	}

	// The provider's token travels to an HTTPS remote's host, so a host that merely contains
	// the forge's name is another server.
	for _, scenario := range []struct{ name, rawURL string }{
		{"an HTTPS host that starts with dev.azure.com", "https://dev.azure.com.evil.example/org/project/_git/repo"},
		{"an HTTPS host that ends with dev.azure.com", "https://notdev.azure.com/org/project/_git/repo"},
		{"an HTTPS host that starts with a visualstudio.com host", "https://org.visualstudio.com.evil.example/project/_git/repo"},
		{"an HTTPS host two labels under visualstudio.com", "https://org.team.visualstudio.com/project/_git/repo"},
		{"an HTTPS remote that names dev.azure.com in its path", "https://evil.example/dev.azure.com/org/project/_git/repo"},
		{"a remote on another forge", "https://github.com/org/repo.git"},
	} {
		t.Run("should not match when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			provider := azuredevops.NewProvider("token")

			// when
			result := provider.MatchesURL(scenario.rawURL)

			// then
			assert.False(t, result)
		})
	}
}

func TestProviderCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should embed token in clone URL when remote URL exists", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("$test-token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithProject("my-project").
			WithName("my-repo").
			WithRemoteURL("https://dev.azure.com/my-org/my-project/_git/my-repo").
			Build().(globalEntities.Repository)

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://pat:$test-token@dev.azure.com/my-org/my-project/_git/my-repo", result)
	})

	t.Run("should construct clone URL when remote URL is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("$test-token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithProject("my-project").
			WithName("my-repo").
			WithRemoteURL("").
			Build().(globalEntities.Repository)

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://pat:$test-token@dev.azure.com/my-org/my-project/_git/my-repo", result)
	})
}

func TestProviderSSHCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should return SSH URL with alias including project path", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithProject("my-project").
			WithName("my-repo").
			Build().(globalEntities.Repository)

		// when
		result := provider.SSHCloneURL(repo, "acme")

		// then
		assert.Equal(t, "git@dev.azure.com-acme:v3/my-org/my-project/my-repo", result)
	})

	t.Run("should return SSH URL with default host when alias is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := azuredevops.NewProvider("token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithProject("my-project").
			WithName("my-repo").
			Build().(globalEntities.Repository)

		// when
		result := provider.SSHCloneURL(repo, "")

		// then
		assert.Equal(t, "git@ssh.dev.azure.com:v3/my-org/my-project/my-repo", result)
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
