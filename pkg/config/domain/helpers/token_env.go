package helpers

import (
	"os"

	"github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// tokenEnvVars maps each ServiceType to the environment variable names
// checked for authentication tokens, ordered by priority.
var tokenEnvVars = map[entities.ServiceType][]string{ //nolint:gochecknoglobals // read-only lookup table
	entities.GITHUB:      {"GITHUB_TOKEN", "GH_TOKEN"},
	entities.GITLAB:      {"GITLAB_TOKEN", "GL_TOKEN"},
	entities.AZUREDEVOPS: {"AZURE_DEVOPS_EXT_PAT", "SYSTEM_ACCESSTOKEN"},
}

// tokenEnvHints maps each ServiceType to a human-readable string listing
// the environment variables that can provide an authentication token.
var tokenEnvHints = map[entities.ServiceType]string{ //nolint:gochecknoglobals // read-only lookup table
	entities.GITHUB:      "GITHUB_TOKEN or GH_TOKEN",
	entities.GITLAB:      "GITLAB_TOKEN or GL_TOKEN",
	entities.AZUREDEVOPS: "AZURE_DEVOPS_EXT_PAT or SYSTEM_ACCESSTOKEN",
}

// ResolveTokenFromEnv returns the first non-empty token found in
// provider-specific environment variables for the given service type.
// Returns an empty string if no token is found.
//
// Exported for use by autobump and autoupdate to resolve authentication
// tokens from the environment when no explicit token is configured.
func ResolveTokenFromEnv(serviceType entities.ServiceType) string {
	for _, envVar := range tokenEnvVars[serviceType] {
		if token := os.Getenv(envVar); token != "" {
			return token
		}
	}
	return ""
}

// TokenEnvHint returns a human-readable string listing the environment
// variables checked for the given service type (e.g. "GITHUB_TOKEN or GH_TOKEN").
// Returns "<unknown provider>" for unsupported service types.
//
// Exported for use by autobump and autoupdate to produce helpful error
// messages when no token is available.
func TokenEnvHint(serviceType entities.ServiceType) string {
	if hint, ok := tokenEnvHints[serviceType]; ok {
		return hint
	}
	return "<unknown provider>"
}
