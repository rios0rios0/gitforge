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

// gitlabServerVariable is an environment variable that names the GitLab server a token
// exported next to it belongs to, and how its value names that server.
type gitlabServerVariable struct {
	name string
	// server returns the host the value names, "" when the value cannot be read (which
	// counts as another server), and false when the value names no server at all.
	server func(value string) (string, bool)
}

// gitlabServerVariables are the variables that tools which read GITLAB_TOKEN or GL_TOKEN
// take their GitLab server from: glab (GITLAB_HOST, GITLAB_URI, GL_HOST, GITLAB_API_HOST and
// GITLAB_REPO), semantic-release's GitLab plugin (GL_URL, GITLAB_URL), Lerna (GL_API_URL),
// Terraform's gitlab provider (GITLAB_BASE_URL), and a GitLab CI job, whose server
// semantic-release, go-semantic-release and glab fall back to (CI_SERVER_URL, CI_SERVER_HOST,
// CI_SERVER_FQDN, CI_API_V4_URL). A token exported beside one that names a self-managed
// server is that server's and must not be sent to gitlab.com. The check is best effort: a
// tool can also take its server from its own configuration file, which no variable shows.
//
//nolint:gochecknoglobals // read-only lookup table
var gitlabServerVariables = []gitlabServerVariable{
	{"GITLAB_HOST", serverHost},
	{"GITLAB_URI", serverHost},
	{"GL_HOST", serverHost},
	{"GITLAB_API_HOST", serverHost},
	{"GITLAB_REPO", repositoryHost},
	{"GL_URL", serverHost},
	{"GITLAB_URL", serverHost},
	{"GL_API_URL", serverHost},
	{"GITLAB_BASE_URL", serverHost},
	{"CI_SERVER_URL", serverHost},
	{"CI_SERVER_HOST", serverHost},
	{"CI_SERVER_FQDN", serverHost},
	{"CI_API_V4_URL", serverHost},
}

// EnvCredentialResolver reads a forge's token from the environment variables gitforge
// knows for it (configHelpers.TokenEnvVars), in priority order: GH_TOKEN then
// GITHUB_TOKEN (gh's own precedence, so a token gh would use wins here too), GITLAB_TOKEN
// then GL_TOKEN, AZURE_DEVOPS_EXT_PAT then SYSTEM_ACCESSTOKEN, and CODEBERG_TOKEN.
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
			if err := CheckGitLabTokenServer(lookup, name); err != nil {
				return entities.Credential{}, err
			}
		}
		return entities.Credential{Token: token, Source: name + " environment variable"}, nil
	}
	return entities.Credential{}, newUnavailableError(nil, "%s not set", describeUnsetVariables(names))
}

// CheckGitLabTokenServer returns an error matching entities.ErrNoCredential when a variable that
// names a GitLab server names one other than gitlab.com, or holds a value it cannot read: the
// GitLab token in tokenVariable then belongs to that server and must not be sent to gitlab.com.
// The variables are those of glab, semantic-release, Lerna, Terraform's gitlab provider and a
// GitLab CI job (see the README). It returns nil when none of them does. EnvCredentialResolver
// applies it to GITLAB_TOKEN and GL_TOKEN; a consumer applies it to another GitLab token it
// exported itself. A GitLab CI job's own CI_JOB_TOKEN is judged by CheckGitLabCIJobTokenServer
// instead. A nil lookup reads the process environment.
func CheckGitLabTokenServer(lookup func(string) string, tokenVariable string) error {
	if lookup == nil {
		lookup = os.Getenv
	}
	if variable, host, foreign := foreignGitLabHost(lookup); foreign {
		return foreignGitLabHostError(variable, host, tokenVariable)
	}
	return nil
}

// foreignGitLabHost reports the first of gitlabServerVariables that names a server other than
// gitlab.com, with that server's host name, or "" when its value cannot be read.
func foreignGitLabHost(lookup func(string) string) (string, string, bool) {
	for _, variable := range gitlabServerVariables {
		value := strings.TrimSpace(lookup(variable.name))
		if value == "" {
			continue
		}
		host, named := variable.server(value)
		if named && !strings.EqualFold(host, gitlabHost) {
			return variable.name, host, true
		}
	}
	return "", "", false
}

// foreignGitLabHostError explains why the token in tokenVariable is not used. It names the
// host only, never the variable's value: a URL can carry a user name and a password of its
// own, and a value that does not parse cannot be shown safely at all.
func foreignGitLabHostError(variable, host, tokenVariable string) error {
	if host == "" {
		return newUnavailableError(
			nil, "%s does not name %s, so %s is not sent there", variable, gitlabHost, tokenVariable,
		)
	}
	return newUnavailableError(
		nil, "%s names the GitLab server %s, so %s is not sent to %s", variable, host, tokenVariable, gitlabHost,
	)
}

// serverHost reads a value that names a server: a bare host, a host and port, or a URL. It
// returns "" when the value has no host it can read, which refuses the token rather than
// sending it on a guess.
func serverHost(value string) (string, bool) {
	withScheme := value
	if !strings.Contains(value, "://") {
		withScheme = "https://" + value
	}
	if parsed, err := url.Parse(withScheme); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname(), true
	}
	return "", true
}

// repositoryHost reads glab's GITLAB_REPO, which names a server only as a URL
// (`https://gitlab.example.com/group/project`) or in git's scp-like form
// (`git@gitlab.example.com:group/project.git`). A path such as `group/project` names none.
func repositoryHost(value string) (string, bool) {
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil && parsed.Hostname() != "" {
			return parsed.Hostname(), true
		}
		return "", true
	}
	colon := strings.Index(value, ":")
	if colon <= 0 || strings.Contains(value[:colon], "/") {
		return "", false
	}
	host := value[:colon]
	pathStart, _, _ := strings.Cut(value[colon+1:], "/")
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	} else if strings.Contains(pathStart, "@") {
		// `user:password@host:path` is no git address, and what precedes its first colon is a
		// user name or a token, never a host: it is refused without being shown.
		return "", true
	}
	if !isHostName(host) {
		return "", true
	}
	return host, true
}

// isHostName reports whether s is shaped like a host name or an IPv4 address: ASCII letters,
// digits, hyphens and dots, beginning and ending with a letter or a digit.
func isHostName(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		c := s[i]
		alphanumeric := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if alphanumeric {
			continue
		}
		if (c != '-' && c != '.') || i == 0 || i == len(s)-1 {
			return false
		}
	}
	return true
}

// describeUnsetVariables names the variables for "... not set": "CODEBERG_TOKEN is",
// "GH_TOKEN and GITHUB_TOKEN are".
func describeUnsetVariables(names []string) string {
	if len(names) == 1 {
		return names[0] + " is"
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " are"
}
