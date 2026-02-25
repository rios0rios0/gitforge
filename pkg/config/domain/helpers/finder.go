package helpers

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrConfigFileNotFound is returned when no configuration file can be located.
var ErrConfigFileNotFound = errors.New("config file not found")

// FindConfigFile searches for a configuration file in standard locations.
// The appName parameter controls the file name patterns (e.g. "autobump" -> ".autobump.yaml").
// Returns the path to the first file found, or ErrConfigFileNotFound if none is found.
// Exported for use by autobump and autoupdate to locate their configuration files.
func FindConfigFile(appName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	locations := []string{
		".",
		".config",
		"configs",
	}
	if homeDir != "" {
		locations = append(
			locations,
			homeDir,
			filepath.Join(homeDir, ".config"),
		)
	}

	patterns := []string{
		"." + appName + ".yaml",
		"." + appName + ".yml",
		appName + ".yaml",
		appName + ".yml",
	}

	for _, loc := range locations {
		for _, pat := range patterns {
			p := filepath.Join(loc, pat)
			if _, statErr := os.Stat(p); statErr == nil {
				return p, nil
			}
		}
	}

	return "", ErrConfigFileNotFound
}
