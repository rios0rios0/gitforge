package helpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
