package infrastructure

import (
	"context"
	"os"
	"strings"

	configHelpers "github.com/rios0rios0/gitforge/v4/pkg/config/domain/helpers"
	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
)

// EnvCredentialResolver reads a forge's token from the environment variables gitforge
// knows for it (configHelpers.TokenEnvVars), in priority order: GH_TOKEN then
// GITHUB_TOKEN, GITLAB_TOKEN then GL_TOKEN, AZURE_DEVOPS_EXT_PAT then SYSTEM_ACCESSTOKEN,
// and CODEBERG_TOKEN.
type EnvCredentialResolver struct {
	// Lookup reads one variable. Nil means os.Getenv; tests set it instead of changing
	// the process environment, so they can run in parallel.
	Lookup func(name string) string
}

// NewEnvCredentialResolver creates a resolver that reads the process environment.
func NewEnvCredentialResolver() *EnvCredentialResolver {
	return &EnvCredentialResolver{Lookup: os.Getenv}
}

// Resolve returns the first non-empty variable's value, trimmed, with the variable's name
// as the source.
func (r *EnvCredentialResolver) Resolve(
	_ context.Context, request entities.CredentialRequest,
) (entities.Credential, error) {
	names := configHelpers.TokenEnvVars(request.ServiceType)
	if len(names) == 0 {
		return entities.Credential{}, newUnavailableError(
			nil, "no environment variable holds a %s token", request.ServiceType.DisplayName(),
		)
	}

	lookup := r.Lookup
	if lookup == nil {
		lookup = os.Getenv
	}
	for _, name := range names {
		if token := strings.TrimSpace(lookup(name)); token != "" {
			return entities.Credential{Token: token, Source: name + " environment variable"}, nil
		}
	}
	return entities.Credential{}, newUnavailableError(nil, "%s not set", describeUnsetVariables(names))
}

// describeUnsetVariables names the variables for "... not set": "CODEBERG_TOKEN is",
// "GH_TOKEN and GITHUB_TOKEN are".
func describeUnsetVariables(names []string) string {
	if len(names) == 1 {
		return names[0] + " is"
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " are"
}
