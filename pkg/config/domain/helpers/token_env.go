package helpers

import (
	"os"
	"slices"

	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// tokenEnvVars maps each ServiceType to the environment variable names
// checked for authentication tokens, ordered by priority. GitHub's follow gh's
// own precedence, "GH_TOKEN, GITHUB_TOKEN (in order of precedence)" (`gh help
// environment`), so a token gh would use is the token read here.
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
// given service type, in priority order; the slice is empty when there are
// none. It is a copy, so callers may change it.
func TokenEnvVars(serviceType entities.ServiceType) []string {
	return slices.Clone(tokenEnvVars[serviceType])
}

// ResolveTokenFromEnv returns the first non-empty token found in
// provider-specific environment variables for the given service type, in the
// order TokenEnvVars lists them (GH_TOKEN before GITHUB_TOKEN for GitHub).
// Returns an empty string if no token is found.
//
// Deprecated: it makes none of the checks the credentials bounded context
// (pkg/credentials) makes before it hands out an environment token, such as
// refusing a GitLab token while a variable names a server other than
// gitlab.com, and no consumer calls it any more. Use that package's
// EnvCredentialResolver, or NewDefaultCredentialResolver, which then falls
// back to the forge's own CLI.
func ResolveTokenFromEnv(serviceType entities.ServiceType) string {
	for _, envVar := range tokenEnvVars[serviceType] {
		if token := os.Getenv(envVar); token != "" {
			return token
		}
	}
	return ""
}

// TokenEnvHint returns a human-readable string listing the environment
// variables checked for the given service type, in the order they are read
// (e.g. "GH_TOKEN or GITHUB_TOKEN").
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
