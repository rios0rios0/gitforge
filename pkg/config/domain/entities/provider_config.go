package entities

import (
	"os"
	"regexp"
	"strings"

	logger "github.com/sirupsen/logrus"
)

// envVarPattern matches ${VAR_NAME} placeholders in token strings.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)}`)

// ProviderConfig describes a single Git hosting provider instance.
type ProviderConfig struct {
	Type          string   `yaml:"type"`          // "github", "gitlab", "azuredevops"
	Token         string   `yaml:"token"`         // inline, ${ENV_VAR}, or file path
	Organizations []string `yaml:"organizations"` // org names or URLs to scan
}

// ResolveToken expands ${ENV_VAR} references in the token string and,
// if the result is a path to an existing file, reads the token from it.
// Exported for use by autobump and autoupdate when resolving provider credentials.
func (self *ProviderConfig) ResolveToken() string {
	raw := self.Token
	if raw == "" {
		return raw
	}

	resolved := envVarPattern.ReplaceAllStringFunc(raw, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		logger.Warnf("Environment variable %q is not set", varName)
		return ""
	})

	if _, err := os.Stat(resolved); err == nil {
		data, readErr := os.ReadFile(resolved)
		if readErr != nil {
			logger.Warnf("Failed to read token file %q: %v", resolved, readErr)
			return resolved
		}
		logger.Infof("Read token from file %q", resolved)
		return strings.TrimSpace(string(data))
	}

	return resolved
}
