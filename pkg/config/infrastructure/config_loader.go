package infrastructure

import (
	"fmt"

	"gopkg.in/yaml.v3"

	configEntities "github.com/rios0rios0/gitforge/pkg/config/domain/entities"
	"github.com/rios0rios0/gitforge/pkg/config/infrastructure/helpers"
)

// rawConfig is an intermediary struct used to unmarshal the YAML config file.
type rawConfig struct {
	Providers []configEntities.ProviderConfig `yaml:"providers"`
}

// LoadConfig reads and parses the application configuration from the given file path or URL.
// The path may be a local file path or an HTTP/HTTPS URL.
func LoadConfig(path string) (*configEntities.Config, error) {
	data, err := helpers.ReadData(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config from %q: %w", path, err)
	}

	var raw rawConfig
	if err = yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := configEntities.NewConfig(raw.Providers)
	if err = cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}
