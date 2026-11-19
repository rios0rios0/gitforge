package infrastructure_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	"github.com/rios0rios0/gitforge/v4/pkg/credentials/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/test/doubles"
)

const (
	fixtureTenant = "11111111-2222-3333-4444-555555555555"
	adoResource   = "499b84ac-1321-427f-aa17-267ca6975798"
)

func githubRequest() entities.CredentialRequest {
	return entities.CredentialRequest{ServiceType: globalEntities.GITHUB}
}

func azureRequest(organization string) entities.CredentialRequest {
	return entities.CredentialRequest{ServiceType: globalEntities.AZUREDEVOPS, Organization: organization}
}

func TestCLICredentialResolverResolve(t *testing.T) {
	t.Parallel()

	t.Run("should ask gh for the github.com token when gh is installed and signed in", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"gh": "/usr/bin/gh"}, Token: "fixture-gh-token"}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		credential, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-gh-token", credential.Token)
		assert.Equal(t, "gh CLI", credential.Source)
		assert.Equal(t, [][]string{{"/usr/bin/gh", "auth", "token", "--hostname", "github.com"}}, runner.Calls())
	})

	t.Run("should report the CLI as not installed when it is missing from PATH", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		_, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "the gh CLI is not installed")
		assert.Empty(t, runner.Calls())
	})

	t.Run("should report why the CLI cannot be run when PATH lookup refuses it", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{LookPathErr: infrastructure.ErrRelativeExecutable}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		_, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.ErrorIs(t, err, infrastructure.ErrRelativeExecutable)
		assert.Contains(t, err.Error(), "the gh CLI cannot be run")
	})

	t.Run("should name the login command when the CLI cannot provide a token", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{
			Paths:     map[string]string{"gh": "/usr/bin/gh"},
			OutputErr: errors.New("gh auth token --hostname github.com: exit status 1: no oauth token found"),
		}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		_, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		assert.Contains(
			t,
			err.Error(),
			`the gh CLI could not provide a token (authenticate with "gh auth login --hostname github.com")`,
		)
		assert.Contains(t, err.Error(), "no oauth token found")
	})

	t.Run("should decline when the CLI prints an empty token", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"gh": "/usr/bin/gh"}}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		_, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		assert.Contains(t, err.Error(), "the gh CLI returned an empty token")
	})

	t.Run("should refuse the output without echoing it when it is not a single token", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{
			Paths: map[string]string{"gh": "/usr/bin/gh"},
			Token: "fixture-first-line\nfixture-second-line",
		}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		credential, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		assert.Empty(t, credential.Token)
		require.EqualError(t, err, "the gh CLI printed something other than a single token")
	})

	t.Run("should decline when the forge has no supported CLI", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"glab": "/usr/bin/glab"}}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}

		// when
		_, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "GitLab has no supported CLI to ask for a token")
		assert.Empty(t, runner.Calls())
	})

	t.Run("should scope the az token to the organization's tenant when the request names one", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"az": "/usr/bin/az"}, Token: "fixture-az-token"}
		tenants := &doubles.AzureTenantResolverStub{Tenant: fixtureTenant}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		credential, err := resolver.Resolve(t.Context(), azureRequest("fixture-org"))

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-az-token", credential.Token)
		assert.Equal(t, "az CLI", credential.Source)
		assert.Equal(t, []string{"fixture-org"}, tenants.Organizations())
		assert.Equal(t, [][]string{{
			"/usr/bin/az", "account", "get-access-token",
			"--resource", adoResource,
			"--query", "accessToken", "--output", "tsv",
			"--tenant", fixtureTenant,
		}}, runner.Calls())
	})

	t.Run("should keep the default tenant when the organization has none", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"az": "/usr/bin/az"}, Token: "fixture-az-token"}
		tenants := &doubles.AzureTenantResolverStub{}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		_, err := resolver.Resolve(t.Context(), azureRequest("fixture-org"))

		// then
		require.NoError(t, err)
		require.Len(t, runner.Calls(), 1)
		assert.NotContains(t, runner.Calls()[0], "--tenant")
	})

	t.Run("should not look up a tenant when the request names no organization", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"az": "/usr/bin/az"}, Token: "fixture-az-token"}
		tenants := &doubles.AzureTenantResolverStub{Tenant: fixtureTenant}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		_, err := resolver.Resolve(t.Context(), azureRequest(""))

		// then
		require.NoError(t, err)
		assert.Empty(t, tenants.Organizations())
		assert.NotContains(t, runner.Calls()[0], "--tenant")
	})

	t.Run("should report az as not installed before looking up a tenant when az is missing", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{}
		tenants := &doubles.AzureTenantResolverStub{Tenant: fixtureTenant}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		_, err := resolver.Resolve(t.Context(), azureRequest("fixture-org"))

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "the az CLI is not installed")
		assert.Empty(t, tenants.Organizations())
	})

	t.Run("should leave a GitHub request unscoped when it names an organization", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"gh": "/usr/bin/gh"}, Token: "fixture-gh-token"}
		tenants := &doubles.AzureTenantResolverStub{Tenant: fixtureTenant}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}
		request := entities.CredentialRequest{ServiceType: globalEntities.GITHUB, Organization: "fixture-org"}

		// when
		_, err := resolver.Resolve(t.Context(), request)

		// then
		require.NoError(t, err)
		assert.Empty(t, tenants.Organizations())
		assert.Equal(t, [][]string{{"/usr/bin/gh", "auth", "token", "--hostname", "github.com"}}, runner.Calls())
	})

	t.Run("should decline without running az when the tenant lookup fails", func(t *testing.T) {
		t.Parallel()

		// given
		lookupErr := errors.New("connection refused")
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"az": "/usr/bin/az"}, Token: "fixture-az-token"}
		tenants := &doubles.AzureTenantResolverStub{Err: lookupErr}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		_, err := resolver.Resolve(t.Context(), azureRequest("fixture-org"))

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.ErrorIs(t, err, lookupErr)
		assert.Contains(t, err.Error(), `Azure DevOps organization "fixture-org"`)
		assert.Empty(t, runner.Calls())
	})

	t.Run("should refuse a tenant that is not a tenant ID when the lookup returns one", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"az": "/usr/bin/az"}, Token: "fixture-az-token"}
		tenants := &doubles.AzureTenantResolverStub{Tenant: "--allow-everything"}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		_, err := resolver.Resolve(t.Context(), azureRequest("fixture-org"))

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		assert.Contains(t, err.Error(), "is not a tenant ID")
		assert.Empty(t, runner.Calls())
	})

	t.Run("should name the tenant's login command when az fails for the organization", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{
			Paths:     map[string]string{"az": "/usr/bin/az"},
			OutputErr: errors.New("az account get-access-token: exit status 1: AADSTS50076"),
		}
		tenants := &doubles.AzureTenantResolverStub{Tenant: fixtureTenant}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, TenantResolver: tenants}

		// when
		_, err := resolver.Resolve(t.Context(), azureRequest("fixture-org"))

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		assert.Contains(t, err.Error(), `"az login --tenant `+fixtureTenant+` --allow-no-subscriptions"`)
	})

	t.Run("should give up when the CLI outlives the timeout", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"gh": "/usr/bin/gh"}, Block: true}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner, Timeout: 20 * time.Millisecond}

		// when
		_, err := resolver.Resolve(t.Context(), githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, err.Error(), "the gh CLI did not provide a token in time")
		assert.NotContains(t, err.Error(), "authenticate with")
	})

	t.Run("should say the request was cancelled when the caller cancels it", func(t *testing.T) {
		t.Parallel()

		// given
		runner := &doubles.CLIRunnerStub{Paths: map[string]string{"gh": "/usr/bin/gh"}, Block: true}
		resolver := &infrastructure.CLICredentialResolver{Runner: runner}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		// when
		_, err := resolver.Resolve(ctx, githubRequest())

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.ErrorIs(t, err, context.Canceled)
		assert.Contains(t, err.Error(), "asking the gh CLI for a token was cancelled")
		assert.NotContains(t, err.Error(), "authenticate with")
	})
}

func TestNewCLICredentialResolver(t *testing.T) {
	t.Parallel()

	t.Run("should run real CLIs with the default timeout when created with the constructor", func(t *testing.T) {
		t.Parallel()

		// given / when
		resolver := infrastructure.NewCLICredentialResolver()

		// then
		assert.IsType(t, &infrastructure.ExecCLIRunner{}, resolver.Runner)
		assert.IsType(t, &infrastructure.HTTPAzureTenantResolver{}, resolver.TenantResolver)
		assert.Equal(t, infrastructure.DefaultCLITimeout, resolver.Timeout)
	})
}

func TestProviderCLIFor(t *testing.T) {
	t.Parallel()

	t.Run("should describe gh when the forge is GitHub", func(t *testing.T) {
		t.Parallel()

		// given / when
		cli, ok := infrastructure.ProviderCLIFor(globalEntities.GITHUB)

		// then
		require.True(t, ok)
		assert.Equal(t, "gh", cli.Binary)
		assert.Equal(t, "gh auth login --hostname github.com", cli.LoginHint)
	})

	t.Run("should report no CLI when the forge is GitLab", func(t *testing.T) {
		t.Parallel()

		// given / when
		_, ok := infrastructure.ProviderCLIFor(globalEntities.GITLAB)

		// then
		assert.False(t, ok)
	})

	t.Run("should return a copy of the arguments when the caller changes them", func(t *testing.T) {
		t.Parallel()

		// given
		cli, _ := infrastructure.ProviderCLIFor(globalEntities.AZUREDEVOPS)

		// when
		cli.TokenArgs[0] = "changed"

		// then
		fresh, _ := infrastructure.ProviderCLIFor(globalEntities.AZUREDEVOPS)
		assert.Equal(t, "account", fresh.TokenArgs[0])
	})
}
