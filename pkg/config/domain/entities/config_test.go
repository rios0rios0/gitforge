//go:build unit

package entities_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/pkg/config/domain/entities"
	"github.com/rios0rios0/gitforge/test/builders"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when providers are valid", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := entities.NewConfig([]entities.ProviderConfig{
			builders.NewProviderConfigBuilder().
				WithType("github").
				WithToken("ghp_test").
				WithOrganizations([]string{"my-org"}).
				Build().(entities.ProviderConfig),
		})

		// when
		err := cfg.Validate()

		// then
		assert.NoError(t, err)
	})

	t.Run("should return error when type is missing", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := entities.NewConfig([]entities.ProviderConfig{
			builders.NewProviderConfigBuilder().
				WithType("").
				WithToken("ghp_test").
				WithOrganizations([]string{"my-org"}).
				Build().(entities.ProviderConfig),
		})

		// when
		err := cfg.Validate()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, entities.ErrConfigKeyMissing)
	})

	t.Run("should return error when token is missing", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := entities.NewConfig([]entities.ProviderConfig{
			builders.NewProviderConfigBuilder().
				WithType("github").
				WithToken("").
				WithOrganizations([]string{"my-org"}).
				Build().(entities.ProviderConfig),
		})

		// when
		err := cfg.Validate()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, entities.ErrConfigKeyMissing)
	})

	t.Run("should return error when organizations are empty", func(t *testing.T) {
		t.Parallel()

		// given
		cfg := entities.NewConfig([]entities.ProviderConfig{
			builders.NewProviderConfigBuilder().
				WithType("github").
				WithToken("ghp_test").
				WithOrganizations([]string{}).
				Build().(entities.ProviderConfig),
		})

		// when
		err := cfg.Validate()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, entities.ErrConfigKeyMissing)
	})
}

func TestProviderConfigResolveToken(t *testing.T) {
	t.Parallel()

	t.Run("should return empty string when token is empty", func(t *testing.T) {
		t.Parallel()

		// given
		p := builders.NewProviderConfigBuilder().WithToken("").Build().(entities.ProviderConfig)

		// when
		result := p.ResolveToken()

		// then
		assert.Empty(t, result)
	})

	t.Run("should return empty for unset environment variable", func(t *testing.T) {
		t.Parallel()

		// given
		p := builders.NewProviderConfigBuilder().WithToken("${GITFORGE_NONEXISTENT_VAR_12345}").Build().(entities.ProviderConfig)

		// when
		result := p.ResolveToken()

		// then
		assert.Empty(t, result)
	})

	t.Run("should return inline token when not a file path", func(t *testing.T) {
		t.Parallel()

		// given
		p := builders.NewProviderConfigBuilder().WithToken("ghp_abc123").Build().(entities.ProviderConfig)

		// when
		result := p.ResolveToken()

		// then
		assert.Equal(t, "ghp_abc123", result)
	})

	t.Run("should read token from file when path exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "token.txt")
		err := os.WriteFile(tokenFile, []byte("  file-token  \n"), 0o600)
		require.NoError(t, err)
		p := builders.NewProviderConfigBuilder().WithToken(tokenFile).Build().(entities.ProviderConfig)

		// when
		result := p.ResolveToken()

		// then
		assert.Equal(t, "file-token", result)
	})
}

func TestProviderConfigResolveTokenEnvVar(t *testing.T) {
	// given — cannot use t.Parallel with t.Setenv
	t.Setenv("GITFORGE_TEST_TOKEN", "my-secret-token")
	p := builders.NewProviderConfigBuilder().WithToken("${GITFORGE_TEST_TOKEN}").Build().(entities.ProviderConfig)

	// when
	result := p.ResolveToken()

	// then
	assert.Equal(t, "my-secret-token", result)
}
