package helpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/gitforge/v4/pkg/config/domain/helpers"
	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// TestResolveTokenFromEnv is deliberately not parallel: it sets process environment
// variables with t.Setenv, which the runtime forbids in a parallel test.
func TestResolveTokenFromEnv(t *testing.T) {
	t.Run("should return GH_TOKEN when both GitHub variables are set", func(t *testing.T) {
		// given
		t.Setenv("GH_TOKEN", "fixture-gh-token")
		t.Setenv("GITHUB_TOKEN", "fixture-github-token")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITHUB)

		// then
		assert.Equal(t, "fixture-gh-token", token)
	})

	t.Run("should fall back to GITHUB_TOKEN when GH_TOKEN is empty", func(t *testing.T) {
		// given
		t.Setenv("GH_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "fixture-github-token")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITHUB)

		// then
		assert.Equal(t, "fixture-github-token", token)
	})

	t.Run("should return GITLAB_TOKEN when set", func(t *testing.T) {
		// given
		t.Setenv("GITLAB_TOKEN", "gl-token-value")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITLAB)

		// then
		assert.Equal(t, "gl-token-value", token)
	})

	t.Run("should fall back to GL_TOKEN when GITLAB_TOKEN is empty", func(t *testing.T) {
		// given
		t.Setenv("GITLAB_TOKEN", "")
		t.Setenv("GL_TOKEN", "gl-fallback")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITLAB)

		// then
		assert.Equal(t, "gl-fallback", token)
	})

	t.Run("should return AZURE_DEVOPS_EXT_PAT when set", func(t *testing.T) {
		// given
		t.Setenv("AZURE_DEVOPS_EXT_PAT", "ado-pat")

		// when
		token := helpers.ResolveTokenFromEnv(entities.AZUREDEVOPS)

		// then
		assert.Equal(t, "ado-pat", token)
	})

	t.Run("should fall back to SYSTEM_ACCESSTOKEN when AZURE_DEVOPS_EXT_PAT is empty", func(t *testing.T) {
		// given
		t.Setenv("AZURE_DEVOPS_EXT_PAT", "")
		t.Setenv("SYSTEM_ACCESSTOKEN", "ado-system")

		// when
		token := helpers.ResolveTokenFromEnv(entities.AZUREDEVOPS)

		// then
		assert.Equal(t, "ado-system", token)
	})

	t.Run("should return empty string when no env var is set", func(t *testing.T) {
		// given
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITHUB)

		// then
		assert.Empty(t, token)
	})
}

func TestResolveTokenFromEnvWithoutVariables(t *testing.T) {
	t.Parallel()

	t.Run("should return empty string for unknown service type", func(t *testing.T) {
		t.Parallel()

		// given
		unknownType := entities.UNKNOWN

		// when
		token := helpers.ResolveTokenFromEnv(unknownType)

		// then
		assert.Empty(t, token)
	})
}

func TestTokenEnvHint(t *testing.T) {
	t.Parallel()

	t.Run("should return GitHub env var names", func(t *testing.T) {
		t.Parallel()

		// given / when
		hint := helpers.TokenEnvHint(entities.GITHUB)

		// then
		assert.Equal(t, "GH_TOKEN or GITHUB_TOKEN", hint)
	})

	t.Run("should return GitLab env var names", func(t *testing.T) {
		t.Parallel()

		// given / when
		hint := helpers.TokenEnvHint(entities.GITLAB)

		// then
		assert.Equal(t, "GITLAB_TOKEN or GL_TOKEN", hint)
	})

	t.Run("should return Azure DevOps env var names", func(t *testing.T) {
		t.Parallel()

		// given / when
		hint := helpers.TokenEnvHint(entities.AZUREDEVOPS)

		// then
		assert.Equal(t, "AZURE_DEVOPS_EXT_PAT or SYSTEM_ACCESSTOKEN", hint)
	})

	t.Run("should return unknown provider for unsupported type", func(t *testing.T) {
		t.Parallel()

		// given / when
		hint := helpers.TokenEnvHint(entities.UNKNOWN)

		// then
		assert.Equal(t, "<unknown provider>", hint)
	})
}

func TestTokenEnvVars(t *testing.T) {
	t.Parallel()

	t.Run("should list GH_TOKEN before GITHUB_TOKEN when the forge is GitHub", func(t *testing.T) {
		t.Parallel()

		// given
		serviceType := entities.GITHUB

		// when
		names := helpers.TokenEnvVars(serviceType)

		// then
		assert.Equal(t, []string{"GH_TOKEN", "GITHUB_TOKEN"}, names)
	})

	t.Run("should return no variable when the forge has none", func(t *testing.T) {
		t.Parallel()

		// given
		serviceType := entities.BITBUCKET

		// when
		names := helpers.TokenEnvVars(serviceType)

		// then
		assert.Empty(t, names)
	})

	t.Run("should return a copy when the caller changes the result", func(t *testing.T) {
		t.Parallel()

		// given
		names := helpers.TokenEnvVars(entities.GITHUB)

		// when
		names[0] = "CHANGED"

		// then
		assert.Equal(t, []string{"GH_TOKEN", "GITHUB_TOKEN"}, helpers.TokenEnvVars(entities.GITHUB))
	})
}
