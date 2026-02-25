package entities

import (
	"errors"
	"fmt"
)

var (
	ErrConfigKeyMissing = errors.New("config keys missing")
)

// Config holds the full application configuration loaded from a YAML file.
// Exported for use by autobump and autoupdate as the canonical configuration entity.
type Config struct {
	Providers []ProviderConfig
}

// NewConfig creates a Config from a slice of provider configurations.
func NewConfig(providers []ProviderConfig) *Config {
	return &Config{Providers: providers}
}

// Validate checks that all provider entries are complete and returns an error if any are invalid.
func (self *Config) Validate() error {
	for i, p := range self.Providers {
		if p.Type == "" {
			return fmt.Errorf(
				"%w: providers[%d].type is required",
				ErrConfigKeyMissing, i,
			)
		}
		if p.Token == "" {
			return fmt.Errorf(
				"%w: providers[%d].token is required (set inline, via ${ENV_VAR}, or as file path)",
				ErrConfigKeyMissing, i,
			)
		}
		if len(p.Organizations) == 0 {
			return fmt.Errorf(
				"%w: providers[%d].organizations must have at least one entry",
				ErrConfigKeyMissing, i,
			)
		}
	}
	return nil
}
