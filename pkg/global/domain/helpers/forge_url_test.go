package helpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/helpers"
)

func TestHTTPURLHost(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name, rawURL, host string
	}{
		{"an https URL", "https://github.com/org/repo.git", "github.com"},
		{"an http URL", "http://github.com/org/repo.git", "github.com"},
		{"a URL with a user name and a port", "https://fixture-user@GitHub.com:443/org/repo.git", "github.com"},
		{"a URL whose user name looks like a host", "https://github.com@evil.example/org/repo.git", "evil.example"},
		{"a URL with the scheme in capitals", "HTTPS://github.com/org/repo.git", "github.com"},
		{"a URL with surrounding spaces", "  https://github.com/org/repo.git\n", "github.com"},
	} {
		t.Run("should return the host when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			rawURL := scenario.rawURL

			// when
			host, isHTTP := helpers.HTTPURLHost(rawURL)

			// then
			assert.True(t, isHTTP)
			assert.Equal(t, scenario.host, host)
		})
	}

	for _, scenario := range []struct {
		name, rawURL string
	}{
		{"a port that is not a number", "https://github.com:not-a-port/org/repo.git"},
		{"an escaped host", "https://github%2ecom/org/repo.git"},
		{"no host", "https:github.com/org/repo.git"},
	} {
		t.Run("should name no host when an HTTP(S) URL has "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			rawURL := scenario.rawURL

			// when
			host, isHTTP := helpers.HTTPURLHost(rawURL)

			// then
			assert.True(t, isHTTP)
			assert.Empty(t, host)
		})
	}

	for _, scenario := range []struct {
		name, rawURL string
	}{
		{"a scp-like SSH remote", "git@github.com:org/repo.git"},
		{"an SSH host alias", "git@github.com-work:org/repo.git"},
		{"an ssh:// remote", "ssh://git@github.com/org/repo.git"},
		{"a git:// remote", "git://github.com/org/repo.git"},
		{"a file:// remote", "file:///srv/git/repo.git"},
		{"a local path", "/home/fixture-user/Development/github.com/org/repo"},
	} {
		t.Run("should report no HTTP(S) URL when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			rawURL := scenario.rawURL

			// when
			host, isHTTP := helpers.HTTPURLHost(rawURL)

			// then
			assert.False(t, isHTTP)
			assert.Empty(t, host)
		})
	}
}

func TestHTTPRemoteServiceType(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		rawURL      string
		serviceType entities.ServiceType
	}{
		{"https://github.com/org/repo.git", entities.GITHUB},
		{"https://www.github.com/org/repo.git", entities.GITHUB},
		{"https://gitlab.com/group/repo.git", entities.GITLAB},
		{"https://www.gitlab.com/group/repo.git", entities.GITLAB},
		{"https://dev.azure.com/org/project/_git/repo", entities.AZUREDEVOPS},
		{"https://org.visualstudio.com/project/_git/repo", entities.AZUREDEVOPS},
		{"https://codeberg.org/owner/repo.git", entities.CODEBERG},
		{"https://bitbucket.org/org/repo.git", entities.BITBUCKET},
		{"https://www.bitbucket.org/org/repo.git", entities.BITBUCKET},
		{"https://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo", entities.CODECOMMIT},
		{"https://git-codecommit-fips.us-east-2.amazonaws.com/v1/repos/repo", entities.CODECOMMIT},
		{"https://git-codecommit.cn-north-1.amazonaws.com.cn/v1/repos/repo", entities.CODECOMMIT},
		{"https://github.com.evil.example/org/repo.git", entities.UNKNOWN},
		{"https://github.company.com/org/repo.git", entities.UNKNOWN},
		{"https://org.team.visualstudio.com/project/_git/repo", entities.UNKNOWN},
		{"https://github.com:not-a-port/org/repo.git", entities.UNKNOWN},
	} {
		t.Run("should return "+scenario.serviceType.DisplayName()+" when the remote is "+scenario.rawURL,
			func(t *testing.T) {
				t.Parallel()

				// given
				rawURL := scenario.rawURL

				// when
				serviceType, isHTTP := helpers.HTTPRemoteServiceType(rawURL)

				// then
				assert.True(t, isHTTP)
				assert.Equal(t, scenario.serviceType, serviceType)
			})
	}

	t.Run("should leave a remote that is no HTTP(S) URL to the caller when it is an SSH remote", func(t *testing.T) {
		t.Parallel()

		// given
		rawURL := "git@github.com:org/repo.git"

		// when
		serviceType, isHTTP := helpers.HTTPRemoteServiceType(rawURL)

		// then
		assert.False(t, isHTTP)
		assert.Equal(t, entities.UNKNOWN, serviceType)
	})
}

func TestVisualStudioOrganization(t *testing.T) {
	t.Parallel()

	t.Run("should return the organization as written when the host is an organization's", func(t *testing.T) {
		t.Parallel()

		// given
		host := "MyOrg.VisualStudio.com"

		// when
		organization, isLegacy := helpers.VisualStudioOrganization(host)

		// then
		assert.True(t, isLegacy)
		assert.Equal(t, "MyOrg", organization)
	})

	for _, host := range []string{"visualstudio.com", "org.team.visualstudio.com", "dev.azure.com", "visualstudio.com.evil.example"} {
		t.Run("should report no organization when the host is "+host, func(t *testing.T) {
			t.Parallel()

			// given
			candidate := host

			// when
			organization, isLegacy := helpers.VisualStudioOrganization(candidate)

			// then
			assert.False(t, isLegacy)
			assert.Empty(t, organization)
		})
	}
}
