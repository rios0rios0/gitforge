package infrastructure_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
		first := &doubles.CredentialResolverStub{Err: errors.New("GITHUB_TOKEN is not set")}
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

	t.Run("should name the source when it returns an empty token without an error", func(t *testing.T) {
		t.Parallel()

		// given
		empty := &doubles.CredentialResolverStub{Credential: entities.Credential{Source: "fixture source"}}
		chain := infrastructure.NewChainCredentialResolver(empty)

		// when
		_, err := chain.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "no GitHub credential available: the fixture source returned an empty token")
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

// TestNewDefaultCredentialResolverWithAStubGH is deliberately not parallel: it puts a stub gh
// on PATH and sets the GitHub variables with t.Setenv, which the runtime forbids in a parallel
// test.
func TestNewDefaultCredentialResolverWithAStubGH(t *testing.T) {
	t.Run("should take gh's own login, not the GH_TOKEN it declined, when the runner names another server",
		func(t *testing.T) {
			// given
			if runtime.GOOS == "windows" {
				t.Skip("the stub gh is a POSIX shell script")
			}
			dir := t.TempDir()
			// Like gh, the stub prints GH_TOKEN or GITHUB_TOKEN when either reaches it, and the
			// login it stored otherwise.
			stub := "#!/bin/sh\nprintf '%s\\n' \"${GH_TOKEN:-${GITHUB_TOKEN:-fixture-stored-login-placeholder}}\"\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o700))
			t.Setenv("PATH", dir)
			t.Setenv("HOME", t.TempDir())
			t.Setenv("GH_TOKEN", "fixture-declined-token-placeholder")
			t.Setenv("GITHUB_TOKEN", "")
			t.Setenv("GITHUB_SERVER_URL", "https://ghe.example.com")
			resolver := infrastructure.NewDefaultCredentialResolver()

			// when
			credential, err := resolver.Resolve(
				t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITHUB},
			)

			// then
			require.NoError(t, err)
			assert.Equal(t, "fixture-stored-login-placeholder", credential.Token)
			assert.Equal(t, "gh CLI", credential.Source)
		})
}
