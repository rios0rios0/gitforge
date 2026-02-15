package gitlab_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
	"github.com/rios0rios0/gitforge/domain/repositories"
	"github.com/rios0rios0/gitforge/infrastructure/providers/gitlab"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	t.Run("should create provider with given token", func(t *testing.T) {
		t.Parallel()

		// given
		token := "glpat-test123"

		// when
		provider := gitlab.NewProvider(token)

		// then
		require.NotNil(t, provider)
		assert.Equal(t, token, provider.AuthToken())
	})

	t.Run("should create provider with empty token", func(t *testing.T) {
		t.Parallel()

		// given / when
		provider := gitlab.NewProvider("")

		// then
		require.NotNil(t, provider)
	})
}

func TestProviderName(t *testing.T) {
	t.Parallel()

	t.Run("should return gitlab as provider name", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("token")

		// when
		name := provider.Name()

		// then
		assert.Equal(t, "gitlab", name)
	})
}

func TestProviderMatchesURL(t *testing.T) {
	t.Parallel()

	t.Run("should match gitlab.com URLs", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("token")

		// when
		result := provider.MatchesURL("https://gitlab.com/org/repo.git")

		// then
		assert.True(t, result)
	})

	t.Run("should not match non-gitlab URLs", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("token")

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
		provider := gitlab.NewProvider("my-token")
		repo := entities.Repository{
			Organization: "my-org",
			Name:         "my-repo",
			RemoteURL:    "https://gitlab.com/my-org/my-repo.git",
		}

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://oauth2:my-token@gitlab.com/my-org/my-repo.git", result)
	})

	t.Run("should construct clone URL when remote URL is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("my-token")
		repo := entities.Repository{
			Organization: "my-org",
			Name:         "my-repo",
		}

		// when
		result := provider.CloneURL(repo)

		// then
		assert.Equal(t, "https://oauth2:my-token@gitlab.com/my-org/my-repo.git", result)
	})
}

func TestProviderGetServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return GITLAB service type", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("token")

		// when
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
		require.True(t, ok)
		result := lgap.GetServiceType()

		// then
		assert.Equal(t, entities.GITLAB, result)
	})
}

func TestProviderPrepareCloneURL(t *testing.T) {
	t.Parallel()

	t.Run("should return URL unchanged", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("token")
		rawURL := "https://gitlab.com/org/repo.git"

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
		provider := gitlab.NewProvider("my-token")

		// when
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("user")

		// then
		assert.Len(t, methods, 1)
	})

	t.Run("should use oauth2 as default username when empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("my-token")

		// when
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("")

		// then
		assert.Len(t, methods, 1)
	})

	t.Run("should return empty auth methods when token is empty", func(t *testing.T) {
		t.Parallel()

		// given
		provider := gitlab.NewProvider("")

		// when
		lgap, ok := provider.(repositories.LocalGitAuthProvider)
		require.True(t, ok)
		methods := lgap.GetAuthMethods("user")

		// then
		assert.Empty(t, methods)
	})
}
