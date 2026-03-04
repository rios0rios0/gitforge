package helpers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/config"
	log "github.com/sirupsen/logrus"
)

// GetGlobalGitConfig reads the global git configuration file and returns a config.Config object.
// Consumed internally by ReadUserConfig; also exported for direct use by autobump (github.com/rios0rios0/autobump).
func GetGlobalGitConfig() (*config.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get user home directory: %w", err)
	}

	globalConfigPath := filepath.Join(homeDir, ".gitconfig")
	configBytes, err := os.ReadFile(globalConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not read global git config: %w", err)
	}

	cfg := config.NewConfig()

	// Recover from panics in go-git's Config.Unmarshal (known bug with certain git configs)
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("go-git panicked while parsing git config (known bug), using minimal config: %v", r)
		}
	}()

	if err = cfg.Unmarshal(configBytes); err != nil {
		return nil, fmt.Errorf("could not unmarshal global git config: %w", err)
	}

	return cfg, nil
}

// GetOptionFromConfig gets a Git option from local and global Git config.
// Consumed internally by ReadUserConfig; also exported for direct use by autobump (github.com/rios0rios0/autobump).
func GetOptionFromConfig(cfg, globalCfg *config.Config, section string, option string) string {
	opt := cfg.Raw.Section(section).Option(option)
	if opt == "" {
		opt = globalCfg.Raw.Section(section).Option(option)
	}
	return opt
}
