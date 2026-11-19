package helpers

import (
	"os"
	"slices"

	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// tokenEnvVars maps each ServiceType to the environment variable names
// checked for authentication tokens, ordered by priority. GitHub reads
// GH_TOKEN before GITHUB_TOKEN, the order the gh CLI itself uses, so a
// personal token exported for gh wins over the workflow token GitHub
// Actions sets.
var tokenEnvVars = map[entities.ServiceType][]string{ //nolint:gochecknoglobals // read-only lookup table
	entities.UNKNOWN:     {},
	entities.GITHUB:      {"GH_TOKEN", "GITHUB_TOKEN"},
	entities.GITLAB:      {"GITLAB_TOKEN", "GL_TOKEN"},
	entities.AZUREDEVOPS: {"AZURE_DEVOPS_EXT_PAT", "SYSTEM_ACCESSTOKEN"},
	entities.BITBUCKET:   {},
	entities.CODECOMMIT:  {},
	entities.CODEBERG:    {"CODEBERG_TOKEN"},
}

// tokenEnvHints maps each ServiceType to a human-readable string listing
// the environment variables that can provide an authentication token.
var tokenEnvHints = map[entities.ServiceType]string{ //nolint:gochecknoglobals // read-only lookup table
	entities.UNKNOWN:     "",
	entities.GITHUB:      "GH_TOKEN or GITHUB_TOKEN",
	entities.GITLAB:      "GITLAB_TOKEN or GL_TOKEN",
	entities.AZUREDEVOPS: "AZURE_DEVOPS_EXT_PAT or SYSTEM_ACCESSTOKEN",
	entities.BITBUCKET:   "",
	entities.CODECOMMIT:  "",
	entities.CODEBERG:    "CODEBERG_TOKEN",
}

// TokenEnvVars returns the environment variables that can hold a token for the
// given service type, in priority order, or nil when there are none. The slice
// is a copy, so callers may change it.
func TokenEnvVars(serviceType entities.ServiceType) []string {
	return slices.Clone(tokenEnvVars[serviceType])
}

// ResolveTokenFromEnv returns the first non-empty token found in
// provider-specific environment variables for the given service type.
// Returns an empty string if no token is found.
//
// Exported for use by autobump and autoupdate to resolve authentication
// tokens from the environment when no explicit token is configured. The
// credentials bounded context (pkg/credentials) reads the same variables
// and falls back to the forge's own CLI.
func ResolveTokenFromEnv(serviceType entities.ServiceType) string {
	for _, envVar := range tokenEnvVars[serviceType] {
		if token := os.Getenv(envVar); token != "" {
			return token
		}
	}
	return ""
}

// TokenEnvHint returns a human-readable string listing the environment
// variables checked for the given service type (e.g. "GH_TOKEN or GITHUB_TOKEN").
// Returns "<unknown provider>" for unsupported service types.
//
// Exported for use by autobump and autoupdate to produce helpful error
// messages when no token is available.
func TokenEnvHint(serviceType entities.ServiceType) string {
	if hint, ok := tokenEnvHints[serviceType]; ok && hint != "" {
		return hint
	}
	return "<unknown provider>"
}
