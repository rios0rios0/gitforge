package infrastructure

import (
	"context"
	"net/url"
	"os"
	"strings"

	configHelpers "github.com/rios0rios0/gitforge/v4/pkg/config/domain/helpers"
	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// gitlabHost is the only host gitforge's GitLab provider talks to.
const gitlabHost = "gitlab.com"

// gitlabHostVariables name the GitLab server glab sends GITLAB_TOKEN to. A token exported
// for a self-managed server must not be sent to gitlab.com.
//
//nolint:gochecknoglobals // read-only lookup table
var gitlabHostVariables = []string{"GITLAB_HOST", "GL_HOST"}

// EnvCredentialResolver reads a forge's token from the environment variables gitforge
// knows for it (configHelpers.TokenEnvVars), in priority order: GITHUB_TOKEN then
// GH_TOKEN, GITLAB_TOKEN then GL_TOKEN, AZURE_DEVOPS_EXT_PAT then SYSTEM_ACCESSTOKEN,
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
		token := strings.TrimSpace(lookup(name))
		if token == "" {
			continue
		}
		if request.ServiceType == globalEntities.GITLAB {
			if variable, host, foreign := foreignGitLabHost(lookup); foreign {
				return entities.Credential{}, newUnavailableError(
					nil, "%s names the GitLab server %s, so %s is not sent to %s", variable, host, name, gitlabHost,
				)
			}
		}
		return entities.Credential{Token: token, Source: name + " environment variable"}, nil
	}
	return entities.Credential{}, newUnavailableError(nil, "%s not set", describeUnsetVariables(names))
}

// foreignGitLabHost reports the first of GITLAB_HOST and GL_HOST that names a server other
// than gitlab.com, with that server's host name.
func foreignGitLabHost(lookup func(string) string) (string, string, bool) {
	for _, name := range gitlabHostVariables {
		value := strings.TrimSpace(lookup(name))
		if value == "" {
			continue
		}
		host := value
		withScheme := value
		if !strings.Contains(value, "://") {
			withScheme = "https://" + value
		}
		if parsed, err := url.Parse(withScheme); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
		if !strings.EqualFold(host, gitlabHost) {
			return name, host, true
		}
	}
	return "", "", false
}

// describeUnsetVariables names the variables for "... not set": "CODEBERG_TOKEN is",
// "GITHUB_TOKEN and GH_TOKEN are".
func describeUnsetVariables(names []string) string {
	if len(names) == 1 {
		return names[0] + " is"
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " are"
}
