package infrastructure

import (
	"fmt"

	"github.com/go-git/go-git/v5"

	gitHelpers "github.com/rios0rios0/gitforge/pkg/git/infrastructure/helpers"
)

// UserConfig holds user-specific git configuration values read from local and global git config.
// Exported for use by autobump (github.com/rios0rios0/autobump) and autoupdate (github.com/rios0rios0/autoupdate)
// as the canonical way to read user identity and signing settings before committing.
type UserConfig struct {
	Name          string
	Email         string
	SigningKey    string
	SigningFormat string
}

// ReadUserConfig reads user name, email, signing key, and signing format
// from the repository's local config, falling back to the global ~/.gitconfig.
// Exported for use by autobump (github.com/rios0rios0/autobump) and autoupdate (github.com/rios0rios0/autoupdate).
func ReadUserConfig(repo *git.Repository) (*UserConfig, error) {
	localCfg, err := repo.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to read local git config: %w", err)
	}

	globalCfg, err := gitHelpers.GetGlobalGitConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read global git config: %w", err)
	}

	return &UserConfig{
		Name:          gitHelpers.GetOptionFromConfig(localCfg, globalCfg, "user", "name"),
		Email:         gitHelpers.GetOptionFromConfig(localCfg, globalCfg, "user", "email"),
		SigningKey:    gitHelpers.GetOptionFromConfig(localCfg, globalCfg, "user", "signingkey"),
		SigningFormat: gitHelpers.GetOptionFromConfig(localCfg, globalCfg, "gpg", "format"),
	}, nil
}
