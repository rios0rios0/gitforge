package github_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
	"github.com/rios0rios0/gitforge/domain/repositories"
	"github.com/rios0rios0/gitforge/infrastructure/providers/github"
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

	t.Run("should match github.com URLs", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")

		// when
		result := provider.MatchesURL("https://github.com/org/repo.git")

		// then
		assert.True(t, result)
	})

	t.Run("should not match non-github URLs", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")

		// when
		result := provider.MatchesURL("https://gitlab.com/org/repo.git")

		// then
		assert.False(t, result)
	})
}

func TestProviderCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should embed token in clone URL when remote URL exists", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("my-token")
		repo := entities.Repository{
			Organization: "my-org",
			Name:         "my-repo",
			RemoteURL:    "https://github.com/my-org/my-repo.git",
		}

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://x-access-token:my-token@github.com/my-org/my-repo.git", result)
	})

	t.Run("should construct clone URL when remote URL is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("my-token")
		repo := entities.Repository{
			Organization: "my-org",
			Name:         "my-repo",
		}

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://x-access-token:my-token@github.com/my-org/my-repo.git", result)
	})
}

func TestProviderGetServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return GITHUB service type", func(t *testing.T) {
		t.Parallel()

		// given
		provider := github.NewProvider("token")

		// when
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
		require.True(t, ok)
		result := lgap.GetServiceType()

		// then
		assert.Equal(t, entities.GITHUB, result)
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
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
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
		provider := github.NewProvider("my-token")

		// when
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
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
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("user")

		// then
		assert.Empty(t, methods)
	})
}
