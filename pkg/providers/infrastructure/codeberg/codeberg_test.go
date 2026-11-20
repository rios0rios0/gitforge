package codeberg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/gitforge/v4/pkg/providers/infrastructure/codeberg"
)

func TestProviderMatchesURL(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct{ name, rawURL string }{
		{"an HTTPS remote on codeberg.org", "https://codeberg.org/owner/repo.git"},
		{"an HTTPS remote with a user name", "https://token@codeberg.org/owner/repo.git"},
		{"a scp-like SSH remote", "git@codeberg.org:owner/repo.git"},
		{"an SSH host alias", "git@codeberg.org-work:owner/repo.git"},
		{"an ssh:// remote", "ssh://git@codeberg.org/owner/repo.git"},
	} {
		t.Run("should match when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			provider := codeberg.NewProvider("token")

			// when
			result := provider.MatchesURL(scenario.rawURL)

			// then
			assert.True(t, result)
		})
	}

	// The provider's token travels to an HTTPS remote's host, so a host that merely contains
	// the forge's name is another server.
	for _, scenario := range []struct{ name, rawURL string }{
		{"an HTTPS host that starts with codeberg.org", "https://codeberg.org.evil.example/owner/repo.git"},
		{"an HTTPS host that ends with codeberg.org", "https://notcodeberg.org/owner/repo.git"},
		{"another Forgejo server that names codeberg.org in its path", "https://forgejo.example.com/codeberg.org/repo.git"},
		{"a remote on another forge", "https://github.com/org/repo.git"},
	} {
		t.Run("should not match when the remote is "+scenario.name, func(t *testing.T) {
			t.Parallel()

			// given
			provider := codeberg.NewProvider("token")

			// when
			result := provider.MatchesURL(scenario.rawURL)

			// then
			assert.False(t, result)
		})
	}
}
