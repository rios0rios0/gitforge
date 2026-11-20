package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
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

	t.Run("should parse GitHub SSH URL with SSH config alias", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@github.com-mine:rios0rios0/autobump.git"

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

// TestParseRemoteURLWithSSHScheme covers SSH remotes written as a URL, ssh://[user@]host[:port]/path.
// Each case names the scp-like remote it must parse exactly like: the user and the port pick
// the account and the port to connect with, not the repository.
func TestParseRemoteURLWithSSHScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawURL  string
		scpLike string
		want    infrastructure.RemoteURLInfo
	}{
		{
			name:    "should parse a GitHub ssh:// URL with a user",
			rawURL:  "ssh://git@github.com/rios0rios0/autobump.git",
			scpLike: "git@github.com:rios0rios0/autobump.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITHUB,
				Organization: "rios0rios0",
				RepoName:     "autobump",
			},
		},
		{
			name:    "should parse a GitHub ssh:// URL with a user and a port",
			rawURL:  "ssh://git@github.com:22/rios0rios0/autobump.git",
			scpLike: "git@github.com:rios0rios0/autobump.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITHUB,
				Organization: "rios0rios0",
				RepoName:     "autobump",
			},
		},
		{
			name:    "should parse a GitHub ssh:// URL without a user",
			rawURL:  "ssh://github.com/rios0rios0/autobump.git",
			scpLike: "git@github.com:rios0rios0/autobump.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITHUB,
				Organization: "rios0rios0",
				RepoName:     "autobump",
			},
		},
		{
			name:    "should parse a GitHub ssh:// URL without a user but with a port",
			rawURL:  "ssh://ssh.github.com:443/rios0rios0/autobump",
			scpLike: "git@ssh.github.com:rios0rios0/autobump",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITHUB,
				Organization: "rios0rios0",
				RepoName:     "autobump",
			},
		},
		{
			name:    "should parse a GitHub ssh:// URL naming an SSH config alias",
			rawURL:  "ssh://git@github.com-mine/rios0rios0/autobump.git",
			scpLike: "git@github.com-mine:rios0rios0/autobump.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITHUB,
				Organization: "rios0rios0",
				RepoName:     "autobump",
			},
		},
		{
			name:    "should parse a GitLab ssh:// URL with a user",
			rawURL:  "ssh://git@gitlab.com/mygroup/myrepo.git",
			scpLike: "git@gitlab.com:mygroup/myrepo.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITLAB,
				Organization: "mygroup",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should parse a GitLab ssh:// URL with nested groups, a user and a port",
			rawURL:  "ssh://git@gitlab.com:2222/group/subgroup/myrepo.git",
			scpLike: "git@gitlab.com:group/subgroup/myrepo.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITLAB,
				Organization: "group/subgroup",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should parse a GitLab ssh:// URL with nested groups and without a user",
			rawURL:  "ssh://gitlab.com/group/subgroup/myrepo.git",
			scpLike: "git@gitlab.com:group/subgroup/myrepo.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITLAB,
				Organization: "group/subgroup",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should parse a self-hosted GitLab ssh:// URL with another user and a port",
			rawURL:  "ssh://deploy@gitlab.example.com:2222/mygroup/myrepo.git",
			scpLike: "git@gitlab.example.com:mygroup/myrepo.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITLAB,
				Organization: "mygroup",
				RepoName:     "myrepo",
			},
		},
		{
			// GitLab's documented SSH endpoint on port 443, for networks that block port 22.
			name:    "should parse a GitLab ssh:// URL through altssh.gitlab.com on port 443",
			rawURL:  "ssh://git@altssh.gitlab.com:443/group/repo.git",
			scpLike: "git@gitlab.com:group/repo.git",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.GITLAB,
				Organization: "group",
				RepoName:     "repo",
			},
		},
		{
			name:    "should parse an Azure DevOps ssh:// URL with a user",
			rawURL:  "ssh://git@ssh.dev.azure.com/v3/myorg/myproject/myrepo",
			scpLike: "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.AZUREDEVOPS,
				Organization: "myorg",
				Project:      "myproject",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should parse an Azure DevOps ssh:// URL with a user and a port",
			rawURL:  "ssh://git@ssh.dev.azure.com:22/v3/myorg/myproject/myrepo",
			scpLike: "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.AZUREDEVOPS,
				Organization: "myorg",
				Project:      "myproject",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should parse an Azure DevOps ssh:// URL without a user",
			rawURL:  "ssh://ssh.dev.azure.com/v3/myorg/myproject/myrepo",
			scpLike: "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.AZUREDEVOPS,
				Organization: "myorg",
				Project:      "myproject",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should parse an Azure DevOps ssh:// URL without a user but with a port",
			rawURL:  "ssh://ssh.dev.azure.com:22/v3/myorg/myproject/myrepo",
			scpLike: "git@ssh.dev.azure.com:v3/myorg/myproject/myrepo",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.AZUREDEVOPS,
				Organization: "myorg",
				Project:      "myproject",
				RepoName:     "myrepo",
			},
		},
		{
			name:    "should keep an escaped Azure DevOps project name escaped in an ssh:// URL",
			rawURL:  "ssh://git@ssh.dev.azure.com:22/v3/myorg/My%20Project/myrepo",
			scpLike: "git@ssh.dev.azure.com:v3/myorg/My%20Project/myrepo",
			want: infrastructure.RemoteURLInfo{
				ServiceType:  globalEntities.AZUREDEVOPS,
				Organization: "myorg",
				Project:      "My%20Project",
				RepoName:     "myrepo",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given
			scpLikeResult, err := infrastructure.ParseRemoteURL(test.scpLike)
			require.NoError(t, err)

			// when
			result, err := infrastructure.ParseRemoteURL(test.rawURL)

			// then
			require.NoError(t, err)
			assert.Equal(t, test.want, *result)
			assert.Equal(t, *scpLikeResult, *result, "the ssh:// URL must parse like %s", test.scpLike)
		})
	}

	t.Run("should return error for an ssh:// URL of an unsupported forge", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "ssh://git@bitbucket.org/owner/repo.git"

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
