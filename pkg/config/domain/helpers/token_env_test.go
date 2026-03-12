//go:build unit

package helpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/gitforge/pkg/config/domain/helpers"
	"github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func TestResolveTokenFromEnv(t *testing.T) {
	t.Run("should return GITHUB_TOKEN when set", func(t *testing.T) {
		// given
		t.Setenv("GITHUB_TOKEN", "gh-token-value")
		t.Setenv("GH_TOKEN", "")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITHUB)

		// then
		assert.Equal(t, "gh-token-value", token)
	})

	t.Run("should fall back to GH_TOKEN when GITHUB_TOKEN is empty", func(t *testing.T) {
		// given
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "gh-fallback")

		// when
		token := helpers.ResolveTokenFromEnv(entities.GITHUB)

		// then
		assert.Equal(t, "gh-fallback", token)
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

	t.Run("should return empty string for unknown service type", func(t *testing.T) {
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
		assert.Equal(t, "GITHUB_TOKEN or GH_TOKEN", hint)
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
