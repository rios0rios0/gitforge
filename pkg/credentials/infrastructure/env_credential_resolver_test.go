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

	t.Run("should return GH_TOKEN when both GitHub variables are set, as gh does", func(t *testing.T) {
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

	t.Run("should not send a GitLab token to gitlab.com when GITLAB_HOST names another server", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GITLAB_TOKEN": "fixture-gitlab-token",
			"GITLAB_HOST":  "https://gitlab.example.com:8443/",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err,
			"GITLAB_HOST names the GitLab server gitlab.example.com, so GITLAB_TOKEN is not sent to gitlab.com")
		assert.Empty(t, credential.Token)
	})

	// glab names its server in GITLAB_URI, GITLAB_API_HOST or GITLAB_REPO; semantic-release's
	// GitLab plugin pairs GL_TOKEN and GITLAB_TOKEN with GL_URL or GITLAB_URL, Lerna with
	// GL_API_URL, Terraform's provider with GITLAB_BASE_URL; and a GitLab CI job names its own
	// server, which those tools fall back to.
	for _, scenario := range []struct {
		variable, value, host, tokenVariable string
	}{
		{"GL_HOST", "gitlab.example.com", "gitlab.example.com", "GL_TOKEN"},
		{"GITLAB_URI", "gitlab.example.com", "gitlab.example.com", "GITLAB_TOKEN"},
		{"GITLAB_API_HOST", "api.gitlab.example.com:8443", "api.gitlab.example.com", "GITLAB_TOKEN"},
		{"GITLAB_REPO", "https://gitlab.example.com/group/project", "gitlab.example.com", "GITLAB_TOKEN"},
		{"GITLAB_REPO", "git@gitlab.example.com:group/project.git", "gitlab.example.com", "GITLAB_TOKEN"},
		{"GL_URL", "https://gitlab.example.com/", "gitlab.example.com", "GL_TOKEN"},
		{"GITLAB_URL", "https://gitlab.example.com/gitlab/api/v4", "gitlab.example.com", "GL_TOKEN"},
		{"GL_API_URL", "https://gitlab.example.com/api/v4", "gitlab.example.com", "GL_TOKEN"},
		{"GITLAB_BASE_URL", "https://gitlab.example.com/api/v4/", "gitlab.example.com", "GITLAB_TOKEN"},
		{"CI_SERVER_URL", "https://gitlab.example.com", "gitlab.example.com", "GITLAB_TOKEN"},
		{"CI_SERVER_HOST", "gitlab.example.com", "gitlab.example.com", "GITLAB_TOKEN"},
		{"CI_SERVER_FQDN", "gitlab.example.com:8443", "gitlab.example.com", "GITLAB_TOKEN"},
		{"CI_API_V4_URL", "https://gitlab.example.com/api/v4", "gitlab.example.com", "GL_TOKEN"},
	} {
		t.Run("should not send a GitLab token to gitlab.com when "+scenario.variable+" is "+scenario.value,
			func(t *testing.T) {
				t.Parallel()

				// given
				resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
					scenario.tokenVariable: "fixture-gitlab-token",
					scenario.variable:      scenario.value,
				})}

				// when
				credential, err := resolver.Resolve(
					t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB},
				)

				// then
				require.ErrorIs(t, err, entities.ErrNoCredential)
				require.EqualError(t, err, scenario.variable+" names the GitLab server "+scenario.host+
					", so "+scenario.tokenVariable+" is not sent to gitlab.com")
				assert.Empty(t, credential.Token)
			})
	}

	t.Run("should refuse the GitLab token without showing a server value it cannot read", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GITLAB_TOKEN": "fixture-gitlab-token",
			"GITLAB_URL":   "gitlab.example.com:not-a-port",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err, "GITLAB_URL does not name gitlab.com, so GITLAB_TOKEN is not sent there")
		assert.Empty(t, credential.Token)
	})

	t.Run("should name only the host when a server URL carries a user name", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GITLAB_TOKEN": "fixture-gitlab-token",
			"GITLAB_URL":   "https://fixture-user@gitlab.example.com/",
		})}

		// when
		_, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

		// then
		require.EqualError(t, err,
			"GITLAB_URL names the GitLab server gitlab.example.com, so GITLAB_TOKEN is not sent to gitlab.com")
	})

	for _, scenario := range []struct {
		variable, value string
	}{
		{"GITLAB_REPO", "https://gitlab.example.com:not-a-port/group/project"},
		{"GITLAB_REPO", "fixture-user:x@gitlab.example.com:group/project"},
		{"GITLAB_REPO", "git@[2001:db8::1]:group/project"},
	} {
		t.Run("should refuse the GitLab token without showing "+scenario.variable+" when it cannot read "+
			scenario.value, func(t *testing.T) {
			t.Parallel()

			// given
			resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
				"GITLAB_TOKEN":    "fixture-gitlab-token",
				scenario.variable: scenario.value,
			})}

			// when
			_, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

			// then
			require.ErrorIs(t, err, entities.ErrNoCredential)
			require.EqualError(t, err, scenario.variable+" does not name gitlab.com, so GITLAB_TOKEN is not sent there")
		})
	}

	for _, scenario := range []struct {
		variable, value string
	}{
		{"GITLAB_REPO", "group/subgroup/project"},
		{"GITLAB_REPO", "group/project:x"},
		{"GITLAB_REPO", "https://gitlab.com:443/group/project"},
		{"GITLAB_REPO", "git@gitlab.com:group/project.git"},
		{"CI_SERVER_HOST", "gitlab.com"},
		{"CI_API_V4_URL", "https://gitlab.com/api/v4"},
	} {
		t.Run("should use the GitLab token when "+scenario.variable+" is "+scenario.value, func(t *testing.T) {
			t.Parallel()

			// given
			resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
				"GITLAB_TOKEN":    "fixture-gitlab-token",
				scenario.variable: scenario.value,
			})}

			// when
			credential, err := resolver.Resolve(
				t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB},
			)

			// then
			require.NoError(t, err)
			assert.Equal(t, "fixture-gitlab-token", credential.Token)
		})
	}

	t.Run("should use the GitLab token when GITLAB_URL names gitlab.com", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GL_TOKEN":   "fixture-gitlab-token",
			"GITLAB_URL": "https://gitlab.com/",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-gitlab-token", credential.Token)
	})

	t.Run("should use the GitLab token when GL_HOST names gitlab.com", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GL_TOKEN": "fixture-gitlab-token",
			"GL_HOST":  "GitLab.com:443",
		})}

		// when
		credential, err := resolver.Resolve(t.Context(), entities.CredentialRequest{ServiceType: globalEntities.GITLAB})

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-gitlab-token", credential.Token)
		assert.Equal(t, "GL_TOKEN environment variable", credential.Source)
	})

	t.Run("should ignore GITLAB_HOST when the forge is GitHub", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := &infrastructure.EnvCredentialResolver{Lookup: lookupFrom(map[string]string{
			"GITHUB_TOKEN": "fixture-github-token",
			"GITLAB_HOST":  "gitlab.example.com",
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

func TestCheckGitLabTokenServer(t *testing.T) {
	t.Parallel()

	t.Run("should refuse an exported GitLab token when a variable names another GitLab server", func(t *testing.T) {
		t.Parallel()

		// given
		lookup := lookupFrom(map[string]string{"GITLAB_BASE_URL": "https://gitlab.example.com/api/v4"})

		// when
		err := infrastructure.CheckGitLabTokenServer(lookup, "MY_GITLAB_TOKEN")

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err,
			"GITLAB_BASE_URL names the GitLab server gitlab.example.com, so MY_GITLAB_TOKEN is not sent to gitlab.com")
	})

	t.Run("should allow the token when no variable names a GitLab server", func(t *testing.T) {
		t.Parallel()

		// given
		lookup := lookupFrom(nil)

		// when
		err := infrastructure.CheckGitLabTokenServer(lookup, "MY_GITLAB_TOKEN")

		// then
		require.NoError(t, err)
	})
}

// TestGitLabServerChecksReadTheEnvironment is deliberately not parallel: it sets the
// process environment with t.Setenv, which the runtime forbids in a parallel test.
func TestGitLabServerChecksReadTheEnvironment(t *testing.T) {
	t.Run("should read the process environment when CheckGitLabTokenServer has no lookup", func(t *testing.T) {
		// given
		t.Setenv("GITLAB_HOST", "gitlab.example.com")

		// when
		err := infrastructure.CheckGitLabTokenServer(nil, "MY_GITLAB_TOKEN")

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
	})

	t.Run("should read the process environment when GitLabCIServerHost has no lookup", func(t *testing.T) {
		// given
		for _, name := range []string{"CI_SERVER_URL", "CI_SERVER_FQDN", "CI_API_V4_URL"} {
			t.Setenv(name, "")
		}
		t.Setenv("CI_SERVER_HOST", "gitlab.example.com")

		// when
		host, err := infrastructure.GitLabCIServerHost(nil)

		// then
		require.NoError(t, err)
		assert.Equal(t, "gitlab.example.com", host)
	})
}

func TestGitLabCIServerHost(t *testing.T) {
	t.Parallel()

	t.Run("should return the host every CI server variable agrees on when a job runs", func(t *testing.T) {
		t.Parallel()

		// given
		lookup := lookupFrom(map[string]string{
			"CI_SERVER_URL":  "https://gitlab.example.com:8443",
			"CI_SERVER_HOST": "gitlab.example.com",
			"CI_SERVER_FQDN": "gitlab.example.com:8443",
			"CI_API_V4_URL":  "https://gitlab.example.com:8443/api/v4",
		})

		// when
		host, err := infrastructure.GitLabCIServerHost(lookup)

		// then
		require.NoError(t, err)
		assert.Equal(t, "gitlab.example.com", host)
	})

	t.Run("should ignore the other GitLab server variables when a job runs", func(t *testing.T) {
		t.Parallel()

		// given: GITLAB_HOST describes another token, never the job's
		lookup := lookupFrom(map[string]string{"CI_SERVER_HOST": "gitlab.com", "GITLAB_HOST": "gitlab.example.com"})

		// when
		host, err := infrastructure.GitLabCIServerHost(lookup)

		// then
		require.NoError(t, err)
		assert.Equal(t, "gitlab.com", host)
	})

	for _, scenario := range []struct {
		name    string
		values  map[string]string
		message string
	}{
		{
			"no CI server variable is set",
			nil,
			"none of CI_SERVER_URL, CI_SERVER_HOST, CI_SERVER_FQDN, CI_API_V4_URL names the GitLab server of a CI job",
		},
		{
			"two CI server variables disagree",
			map[string]string{"CI_SERVER_HOST": "gitlab.com", "CI_API_V4_URL": "https://gitlab.example.com/api/v4"},
			"CI_SERVER_HOST names the GitLab server gitlab.com but CI_API_V4_URL names gitlab.example.com",
		},
		{
			"a CI server variable cannot be read",
			map[string]string{"CI_SERVER_FQDN": "gitlab.example.com:not-a-port"},
			"CI_SERVER_FQDN cannot be read as the GitLab server of the CI job",
		},
	} {
		t.Run("should fail when "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			lookup := lookupFrom(scenario.values)

			// when
			host, err := infrastructure.GitLabCIServerHost(lookup)

			// then
			require.ErrorIs(t, err, entities.ErrNoCredential)
			require.EqualError(t, err, scenario.message)
			assert.Empty(t, host)
		})
	}
}

func TestCheckGitLabCIJobTokenServer(t *testing.T) {
	t.Parallel()

	t.Run("should allow CI_JOB_TOKEN when the CI job runs on gitlab.com", func(t *testing.T) {
		t.Parallel()

		// given
		lookup := lookupFrom(map[string]string{"CI_SERVER_HOST": "gitlab.com", "CI_SERVER_URL": "https://gitlab.com"})

		// when
		err := infrastructure.CheckGitLabCIJobTokenServer(lookup)

		// then
		require.NoError(t, err)
	})

	t.Run("should refuse CI_JOB_TOKEN when the CI job runs on another GitLab server", func(t *testing.T) {
		t.Parallel()

		// given
		lookup := lookupFrom(map[string]string{"CI_SERVER_HOST": "gitlab.example.com"})

		// when
		err := infrastructure.CheckGitLabCIJobTokenServer(lookup)

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.EqualError(t, err,
			"the CI job runs on the GitLab server gitlab.example.com, so CI_JOB_TOKEN is not sent to gitlab.com")
	})

	t.Run("should refuse CI_JOB_TOKEN when no CI server variable names the job's server", func(t *testing.T) {
		t.Parallel()

		// given
		lookup := lookupFrom(map[string]string{"GITLAB_HOST": "gitlab.com"})

		// when
		err := infrastructure.CheckGitLabCIJobTokenServer(lookup)

		// then
		require.ErrorIs(t, err, entities.ErrNoCredential)
		require.ErrorContains(t, err, "CI_JOB_TOKEN is not sent to gitlab.com: none of CI_SERVER_URL")
	})
}
