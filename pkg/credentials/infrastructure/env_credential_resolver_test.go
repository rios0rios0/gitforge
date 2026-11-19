package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	"github.com/rios0rios0/gitforge/v4/pkg/credentials/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

func lookupFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestEnvCredentialResolverResolve(t *testing.T) {
	t.Parallel()

	t.Run("should return GH_TOKEN when both GitHub variables are set", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GH_TOKEN":     "fixture-gh-token",
			"GITHUB_TOKEN": "fixture-github-token",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITHUB})

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-gh-token", credential.Token)
		assert.Equal(t, "GH_TOKEN environment variable", credential.Source)
	})

	t.Run("should fall back to GITHUB_TOKEN when GH_TOKEN is empty", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GITHUB_TOKEN": "fixture-github-token",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITHUB})

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-github-token", credential.Token)
		assert.Equal(t, "GITHUB_TOKEN environment variable", credential.Source)
	})

	t.Run("should return AZURE_DEVOPS_EXT_PAT when the forge is Azure DevOps", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"AZURE_DEVOPS_EXT_PAT": "fixture-ado-token",
			"SYSTEM_ACCESSTOKEN":   "fixture-system-token",
		})}

		// when
		credential, err := resolver.Resolve(
			t.Context(), entities.CredentialRequest{ServiceType: globalEntities.AZUREDEVOPS},
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-ado-token", credential.Token)
		assert.Equal(t, "AZURE_DEVOPS_EXT_PAT environment variable", credential.Source)
	})

	t.Run("should trim surrounding whitespace when the variable holds a token", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"CODEBERG_TOKEN": "  fixture-codeberg-token\n",
		})}

		// when
		credential, err := resolver.Resolve(
			t.Context(),
			entities.CredentialRequest{ServiceType: globalEntities.CODEBERG},
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-codeberg-token", credential.Token)
	})

	t.Run("should treat a variable as unset when it holds only whitespace", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GH_TOKEN":     " \n",
			"GITHUB_TOKEN": "fixture-github-token",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITHUB})

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-github-token", credential.Token)
	})

	t.Run("should name every variable when none of them is set", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(nil)}

		// when
		_, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITHUB})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "GH_TOKEN and GITHUB_TOKEN are not set")
	})

	t.Run("should name the single variable when the forge has one", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(nil)}

		// when
		_, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.CODEBERG})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "CODEBERG_TOKEN is not set")
	})

	t.Run("should decline when no variable holds a token for the forge", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(nil)}

		// when
		_, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.BITBUCKET})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "no environment variable holds a Bitbucket token")
	})
}

// TestNewEnvCredentialResolver is deliberately not parallel: it sets a process
// environment variable with t.Setenv, which the runtime forbids in a parallel test.
func TestNewEnvCredentialResolver(t *testing.T) {
	t.Run("should read the process environment when created with the constructor", func(t *testing.T) {
		// given
		t.Setenv("CODEBERG_TOKEN", "fixture-codeberg-token")
		resolver := infrastructure.NewEnvCredentialResolver()

		// when
		credential, err := resolver.Resolve(
			t.Context(),
			entities.CredentialRequest{ServiceType: globalEntities.CODEBERG},
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-codeberg-token", credential.Token)
	})
}
