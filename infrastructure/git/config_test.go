package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitcfg "github.com/go-git/go-git/v5/config"

	"github.com/rios0rios0/gitforge/infrastructure/git"
)

func TestGetGlobalGitConfig(t *testing.T) {
	t.Parallel()

	t.Run("should read global git config when it exists", func(t *testing.T) {
		t.Parallel()

		// given — check if ~/.gitconfig exists; skip if not
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home directory")
		}
		configPath := filepath.Join(homeDir, ".gitconfig")
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			t.Skip("no ~/.gitconfig file found, skipping")
		}

		// when
		cfg, err := git.GetGlobalGitConfig()

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg)
	})
}

func TestGetOptionFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("should return option from local config when present", func(t *testing.T) {
		t.Parallel()

		// given
		localCfg := gitcfg.NewConfig()
		localCfg.Raw.Section("user").SetOption("name", "Local User")

		globalCfg := gitcfg.NewConfig()
		globalCfg.Raw.Section("user").SetOption("name", "Global User")

		// when
		result := git.GetOptionFromConfig(localCfg, globalCfg, "user", "name")

		// then
		assert.Equal(t, "Local User", result)
	})

	t.Run("should fall back to global config when local option is empty", func(t *testing.T) {
		t.Parallel()

		// given
		localCfg := gitcfg.NewConfig()

		globalCfg := gitcfg.NewConfig()
		globalCfg.Raw.Section("user").SetOption("email", "global@example.com")

		// when
		result := git.GetOptionFromConfig(localCfg, globalCfg, "user", "email")

		// then
		assert.Equal(t, "global@example.com", result)
	})

	t.Run("should return empty string when option is not in either config", func(t *testing.T) {
		t.Parallel()

		// given
		localCfg := gitcfg.NewConfig()
		globalCfg := gitcfg.NewConfig()

		// when
		result := git.GetOptionFromConfig(localCfg, globalCfg, "user", "signingkey")

		// then
		assert.Empty(t, result)
	})
}
