package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/pkg/git/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func TestParseRemoteURL(t *testing.T) {
	t.Parallel()

	t.Run("should parse GitHub SSH URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@github.com:rios0rios0/autobump.git"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITHUB, result.ServiceType)
		assert.Equal(t, "rios0rios0", result.Organization)
		assert.Equal(t, "autobump", result.RepoName)
		assert.Empty(t, result.Project)
	})

	t.Run("should parse GitHub HTTPS URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://github.com/rios0rios0/autobump.git"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITHUB, result.ServiceType)
		assert.Equal(t, "rios0rios0", result.Organization)
		assert.Equal(t, "autobump", result.RepoName)
		assert.Empty(t, result.Project)
	})

	t.Run("should parse GitHub HTTPS URL without .git suffix", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://github.com/rios0rios0/autobump"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITHUB, result.ServiceType)
		assert.Equal(t, "rios0rios0", result.Organization)
		assert.Equal(t, "autobump", result.RepoName)
	})

	t.Run("should parse GitLab SSH URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@gitlab.com:mygroup/myrepo.git"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITLAB, result.ServiceType)
		assert.Equal(t, "mygroup", result.Organization)
		assert.Equal(t, "myrepo", result.RepoName)
		assert.Empty(t, result.Project)
	})

	t.Run("should parse GitLab SSH URL with nested groups", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@gitlab.com:group/subgroup/myrepo.git"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITLAB, result.ServiceType)
		assert.Equal(t, "group/subgroup", result.Organization)
		assert.Equal(t, "myrepo", result.RepoName)
	})

	t.Run("should parse GitLab HTTPS URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://gitlab.com/mygroup/myrepo.git"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITLAB, result.ServiceType)
		assert.Equal(t, "mygroup", result.Organization)
		assert.Equal(t, "myrepo", result.RepoName)
	})

	t.Run("should parse GitLab HTTPS URL with nested groups", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://gitlab.com/group/subgroup/myrepo.git"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITLAB, result.ServiceType)
		assert.Equal(t, "group/subgroup", result.Organization)
		assert.Equal(t, "myrepo", result.RepoName)
	})

	t.Run("should parse Azure DevOps SSH URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.AZUREDEVOPS, result.ServiceType)
		assert.Equal(t, "myorg", result.Organization)
		assert.Equal(t, "myproject", result.Project)
		assert.Equal(t, "myrepo", result.RepoName)
	})

	t.Run("should parse Azure DevOps HTTPS URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://dev.azure.com/myorg/myproject/_git/myrepo"

		// when
		result, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.AZUREDEVOPS, result.ServiceType)
		assert.Equal(t, "myorg", result.Organization)
		assert.Equal(t, "myproject", result.Project)
		assert.Equal(t, "myrepo", result.RepoName)
	})

	t.Run("should return error for empty URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := ""

		// when
		_, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty remote URL")
	})

	t.Run("should return error for unsupported URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://bitbucket.org/owner/repo.git"

		// when
		_, err := infrastructure.ParseRemoteURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported remote URL format")
	})
}

func TestParsePullRequestURL(t *testing.T) {
	t.Parallel()

	t.Run("should parse GitHub PR URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://github.com/rios0rios0/code-guru/pull/42"

		// when
		result, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITHUB, result.ServiceType)
		assert.Equal(t, "rios0rios0", result.Organization)
		assert.Equal(t, "code-guru", result.RepoName)
		assert.Equal(t, 42, result.PRID)
		assert.Empty(t, result.Project)
	})

	t.Run("should parse Azure DevOps PR URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://dev.azure.com/myorg/myproject/_git/myrepo/pullrequest/123"

		// when
		result, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.AZUREDEVOPS, result.ServiceType)
		assert.Equal(t, "myorg", result.Organization)
		assert.Equal(t, "myproject", result.Project)
		assert.Equal(t, "myrepo", result.RepoName)
		assert.Equal(t, 123, result.PRID)
	})

	t.Run("should return error for unsupported host", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://gitlab.com/org/repo/merge_requests/1"

		// when
		_, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported provider host")
	})

	t.Run("should return error for invalid GitHub URL format", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://github.com/org/repo"

		// when
		_, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GitHub PR URL format")
	})

	t.Run("should return error for invalid PR ID", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://github.com/org/repo/pull/abc"

		// when
		_, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR ID")
	})

	t.Run("should return error for empty URL", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := ""

		// when
		_, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty pull request URL")
	})

	t.Run("should return error for invalid Azure DevOps PR URL format", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "https://dev.azure.com/myorg/myproject/myrepo"

		// when
		_, err := infrastructure.ParsePullRequestURL(rawURL)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Azure DevOps PR URL format")
	})
}
