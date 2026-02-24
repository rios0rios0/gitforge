package domain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/gitforge/pkg/config/domain/entities"
)

// envVarPattern matches ${VAR_NAME} placeholders in token strings.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)}`)

var (
	ErrConfigFileNotFound = errors.New("config file not found")
	ErrConfigKeyMissing   = errors.New("config keys missing")
)

// ResolveToken expands ${ENV_VAR} references in a token string and,
// if the result is a path to an existing file, reads the token from it.
func ResolveToken(raw string) string {
	if raw == "" {
		return raw
	}

	// Expand ${ENV_VAR} references
	resolved := envVarPattern.ReplaceAllStringFunc(raw, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		log.Warnf("Environment variable %q is not set", varName)
		return ""
	})

	// If the resolved value is a path to an existing file, read the token from it
	if _, err := os.Stat(resolved); err == nil {
		data, readErr := os.ReadFile(resolved)
		if readErr != nil {
			log.Warnf("Failed to read token file %q: %v", resolved, readErr)
			return resolved
		}
		log.Infof("Read token from file %q", resolved)
		return strings.TrimSpace(string(data))
	}

	return resolved
}

// FindConfigFile searches for a configuration file in standard locations.
// The appName parameter controls the file name patterns (e.g. "autobump" -> ".autobump.yaml").
// Returns the path to the first file found or an error if none is found.
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

// ValidateProviders validates provider configuration entries.
func ValidateProviders(providers []entities.ProviderConfig) error {
	for i, p := range providers {
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
