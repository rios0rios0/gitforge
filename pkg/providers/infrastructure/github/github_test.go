package github_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/pkg/providers/infrastructure/github"
	"github.com/rios0rios0/gitforge/v4/test/builders"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	t.Run("should create provider with given token", func(t *testing.T) {
		t.Parallel()

		// given
		token := "ghp_test123"

		// when
		provider := github.NewProvider(token)

		// then
		require.NotNil(t, provider)
		assert.Equal(t, token, provider.AuthToken())
	})
}

func TestProviderName(t *testing.T) {
	t.Parallel()

	t.Run("should return github as provider name", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")

		// when
		name := provider.Name()

		// then
		assert.Equal(t, "github", name)
	})
}

func TestProviderMatchesURL(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct{ name, rawURL string }{
		{"an HTTPS remote on github.com", "https://github.com/org/repo.git"},
		{"an HTTPS remote on www.github.com", "https://www.github.com/org/repo.git"},
		{"an HTTP remote on github.com", "http://github.com/org/repo.git"},
		{"an HTTPS remote with a user name and a port", "https://x-access-token@github.com:443/org/repo.git"},
		{"an HTTPS remote written in capitals", "HTTPS://GitHub.com/org/repo.git"},
		{"a scp-like SSH remote", "git@github.com:org/repo.git"},
		{"an SSH host alias", "git@github.com-mine:org/repo.git"},
		{"an ssh:// remote", "ssh://git@github.com/org/repo.git"},
		{"an ssh:// remote on GitHub's port 443 host", "ssh://git@ssh.github.com:443/org/repo.git"},
	} {
		t.Run("should match when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			provider := github.NewProvider("token")

			// when
			result := provider.MatchesURL(scenario.rawURL)

			// then
			assert.True(t, result)
		})
	}

	// The provider's token travels to an HTTPS remote's host, so a host that merely contains
	// the forge's name is another server.
	for _, scenario := range []struct{ name, rawURL string }{
		{"an HTTPS host that starts with github.com", "https://github.com.evil.example/org/repo.git"},
		{"an HTTPS host that ends with github.com", "https://notgithub.com/org/repo.git"},
		{"a GitHub Enterprise HTTPS host", "https://github.company.com/org/repo.git"},
		{"an HTTPS remote that names github.com in its path", "https://evil.example/github.com/org/repo.git"},
		{"an HTTPS remote that names github.com as its user", "https://github.com@evil.example/org/repo.git"},
		{"an HTTPS remote whose host cannot be read", "https://github.com:not-a-port/org/repo.git"},
		{"a remote on another forge", "https://gitlab.com/org/repo.git"},
	} {
		t.Run("should not match when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			provider := github.NewProvider("token")

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
		provider := github.NewProvider("$test-token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithName("my-repo").
			WithRemoteURL("https://github.com/my-org/my-repo.git").
			Build().(globalEntities.Repository)

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://x-access-token:$test-token@github.com/my-org/my-repo.git", result)
	})

	t.Run("should construct clone URL when remote URL is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("$test-token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithName("my-repo").
			WithRemoteURL("").
			Build().(globalEntities.Repository)

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://x-access-token:$test-token@github.com/my-org/my-repo.git", result)
	})
}

func TestProviderSSHCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should return SSH URL with alias", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithName("my-repo").
			Build().(globalEntities.Repository)

		// when
		result := provider.SSHCloneURL(repo, "mine")

		// then
		assert.Equal(t, "git@github.com-mine:my-org/my-repo.git", result)
	})

	t.Run("should return SSH URL with default host when alias is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")
		repo := builders.NewRepositoryBuilder().
			WithOrganization("my-org").
			WithName("my-repo").
			Build().(globalEntities.Repository)

		// when
		result := provider.SSHCloneURL(repo, "")

		// then
		assert.Equal(t, "git@github.com:my-org/my-repo.git", result)
	})
}

func TestProviderGetServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return GITHUB service type", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		result := lgap.GetServiceType()

		// then
		assert.Equal(t, globalEntities.GITHUB, result)
	})
}

func TestProviderPrepareCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should return URL unchanged", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")
		rawURL := "https://github.com/org/repo.git"

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
		provider := github.NewProvider("$test-token")

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
		provider := github.NewProvider("")

		// when
		lgap, ok := provider.(globalEntities.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("user")

		// then
		assert.Empty(t, methods)
	})
}
