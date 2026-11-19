package infrastructure_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	"github.com/rios0rios0/gitforge/v4/pkg/credentials/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/test/doubles"
)

func TestChainCredentialResolverResolve(t *testing.T) {
	t.Parallel()

	t.Run("should return the first credential when the first resolver supplies one", func(t *testing.T) {
		t.Parallel()

		// given
		first := &doubles.CredentialResolverStub{
			Credential: entities.Credential{Token: "fixture-first-token", Source: "first source"},
		}
		second := &doubles.CredentialResolverStub{
			Credential: entities.Credential{Token: "fixture-second-token", Source: "second source"},
		}
		chain := infrastructure.NewChainCredentialResolver(first, second)

		// when
		credential, err := chain.Resolve(t.Context(), githubRequest())

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-first-token", credential.Token)
		assert.Equal(t, "first source", credential.Source)
		assert.Empty(t, second.Requests())
	})

	t.Run("should ask the next resolver when one declines", func(t *testing.T) {
		t.Parallel()

		// given
		first := &doubles.CredentialResolverStub{Err: errors.New("GH_TOKEN is not set")}
		second := &doubles.CredentialResolverStub{
			Credential: entities.Credential{Token: "fixture-second-token", Source: "gh CLI"},
		}
		chain := infrastructure.NewChainCredentialResolver(first, second)

		// when
		credential, err := chain.Resolve(t.Context(), githubRequest())

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-second-token", credential.Token)
		assert.Equal(t, []entities.CredentialRequest{githubRequest()}, second.Requests())
	})

	t.Run("should list every reason when all resolvers decline", func(t *testing.T) {
		t.Parallel()

		// given
		chain := infrastructure.NewChainCredentialResolver(
			&doubles.CredentialResolverStub{Err: errors.New("GH_TOKEN and GITHUB_TOKEN are not set")},
			&doubles.CredentialResolverStub{Err: errors.New("the gh CLI is not installed")},
		)

		// when
		_, err := chain.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err,
			"no GitHub credential available: GH_TOKEN and GITHUB_TOKEN are not set; the gh CLI is not installed")
	})

	t.Run("should treat an empty token as a decline when a resolver returns one without an error", func(t *testing.T) {
		t.Parallel()

		// given
		empty := &doubles.CredentialResolverStub{}
		chain := infrastructure.NewChainCredentialResolver(empty)

		// when
		_, err := chain.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		assert.Contains(t, err.Error(), "returned an empty token")
	})

	t.Run("should say no source is configured when the chain is empty", func(t *testing.T) {
		t.Parallel()

		// given
		chain := infrastructure.NewChainCredentialResolver()

		// when
		_, err := chain.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.AZUREDEVOPS})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "no Azure DevOps credential available: no credential source is configured")
	})

	t.Run("should stop asking when the context has ended", func(t *testing.T) {
		t.Parallel()

		// given
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		first := &doubles.CredentialResolverStub{Err: context.Canceled}
		second := &doubles.CredentialResolverStub{
			Credential: entities.Credential{Token: "fixture-second-token", Source: "gh CLI"},
		}
		chain := infrastructure.NewChainCredentialResolver(first, second)

		// when
		_, err := chain.Resolve(ctx, githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, second.Requests())
	})
}

func TestNewDefaultCredentialResolver(t *testing.T) {
	t.Parallel()

	t.Run("should try the environment before the CLI when created", func(t *testing.T) {
		t.Parallel()

		// given / when
		chain := infrastructure.NewDefaultCredentialResolver()

		// then
		require.Len(t, chain.Resolvers, 2)
		assert.IsType(t, &infrastructure.EnvCredentialResolver{}, chain.Resolvers[0])
		assert.IsType(t, &infrastructure.CLICredentialResolver{}, chain.Resolvers[1])
	})
}
