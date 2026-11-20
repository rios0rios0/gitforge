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

// githubAPIHost is the host gitforge's GitHub provider sends its requests to, the API of
// github.com (githubHost).
const githubAPIHost = "api.github.com"

// gheComHostSuffix ends the host of a GHE.com tenant, <tenant>.ghe.com: GitHub Enterprise
// Cloud with data residency.
const gheComHostSuffix = ".ghe.com"

// serverVariable is an environment variable that names the server a token exported next to
// it belongs to, how its value names that server, and the host it names when that server is
// the one gitforge's provider calls.
type serverVariable struct {
	name string
	// server returns the host the value names, "" when the value cannot be read (which
	// counts as another server), and false when the value names no server the token could
	// belong to.
	server func(value string) (string, bool)
	// host is the host the value names when the token is one the provider may be sent;
	// any other host, or a value that cannot be read, keeps the token from the provider.
	host string
}

// githubServerVariables are the variables that say a GitHub token belongs to another server
// than github.com, the only one whose API (api.github.com) gitforge's GitHub provider calls.
// A GitHub Actions runner names its server in GITHUB_SERVER_URL (https://github.com on
// github.com) and GITHUB_API_URL (https://api.github.com): on a GitHub Enterprise Server or
// a GHE.com (*.ghe.com) runner they name that server, which issued the job's GITHUB_TOKEN,
// and a token exported beside them belongs there too. gh's GH_HOST counts only when it names
// a GHE.com host (ghHostServer).
//
//nolint:gochecknoglobals // read-only lookup table
var githubServerVariables = []serverVariable{
	{"GITHUB_SERVER_URL", serverHost, githubHost},
	{"GITHUB_API_URL", serverHost, githubAPIHost},
	{"GH_HOST", ghHostServer, githubHost},
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
var gitlabServerVariables = []serverVariable{
	{"GITLAB_HOST", serverHost, gitlabHost},
	{"GITLAB_URI", serverHost, gitlabHost},
	{"GL_HOST", serverHost, gitlabHost},
	{"GITLAB_API_HOST", serverHost, gitlabHost},
	{"GITLAB_REPO", repositoryHost, gitlabHost},
	{"GL_URL", serverHost, gitlabHost},
	{"GITLAB_URL", serverHost, gitlabHost},
	{"GL_API_URL", serverHost, gitlabHost},
	{"GITLAB_BASE_URL", serverHost, gitlabHost},
	{"CI_SERVER_URL", serverHost, gitlabHost},
	{"CI_SERVER_HOST", serverHost, gitlabHost},
	{"CI_SERVER_FQDN", serverHost, gitlabHost},
	{"CI_API_V4_URL", serverHost, gitlabHost},
}

// tokenServerVariables maps a forge to the variables that can say its environment tokens
// belong to another server. A forge with an empty entry has none, and its tokens are used as
// they are found. Give a forge its variables here when one of its tokens is paired with a
// server in the environment.
//
//nolint:gochecknoglobals // read-only lookup table
var tokenServerVariables = map[globalEntities.ServiceType][]serverVariable{
	globalEntities.UNKNOWN:     nil,
	globalEntities.GITHUB:      githubServerVariables,
	globalEntities.GITLAB:      gitlabServerVariables,
	globalEntities.AZUREDEVOPS: nil,
	globalEntities.BITBUCKET:   nil,
	globalEntities.CODECOMMIT:  nil,
	globalEntities.CODEBERG:    nil,
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
// as the source. It declines that token instead, with an error matching
// entities.ErrNoCredential, when a variable of tokenServerVariables says it belongs to
// another server: a GitHub token while GITHUB_SERVER_URL names a host other than github.com
// or GITHUB_API_URL one other than api.github.com, and a GitLab token while a GitLab server
// variable names a host other than gitlab.com.
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
		if err := checkTokenServer(request.ServiceType, lookup, name); err != nil {
			return entities.Credential{}, err
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
	return checkTokenServer(globalEntities.GITLAB, lookup, tokenVariable)
}

// checkTokenServer returns an error matching entities.ErrNoCredential when one of the forge's
// tokenServerVariables names a host other than its own, or holds a value it cannot read: the
// token in tokenVariable then belongs to another server. It returns nil when none does.
func checkTokenServer(
	serviceType globalEntities.ServiceType, lookup func(string) string, tokenVariable string,
) error {
	if variable, host, foreign := foreignServer(tokenServerVariables[serviceType], lookup); foreign {
		return foreignServerError(serviceType, variable, host, tokenVariable)
	}
	return nil
}

// foreignServer reports the first of variables that names a server other than its host, with
// that server's host name, or "" when its value cannot be read.
func foreignServer(variables []serverVariable, lookup func(string) string) (serverVariable, string, bool) {
	for _, variable := range variables {
		value := strings.TrimSpace(lookup(variable.name))
		if value == "" {
			continue
		}
		host, named := variable.server(value)
		if named && !strings.EqualFold(host, variable.host) {
			return variable, host, true
		}
	}
	return serverVariable{}, "", false
}

// foreignServerError explains why the token in tokenVariable is not used. It names the host
// only, never the variable's value: a URL can carry a user name and a password of its own,
// and a value that does not parse cannot be shown safely at all.
func foreignServerError(
	serviceType globalEntities.ServiceType, variable serverVariable, host, tokenVariable string,
) error {
	if host == "" {
		return newUnavailableError(
			nil, "%s does not name %s, so %s is not sent there", variable.name, variable.host, tokenVariable,
		)
	}
	return newUnavailableError(
		nil, "%s names the %s server %s, so %s is not sent to %s",
		variable.name, serviceType.DisplayName(), host, tokenVariable, variable.host,
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

// ghHostServer reads gh's GH_HOST as serverHost reads a value, and names a server GH_TOKEN and
// GITHUB_TOKEN belong to only when it is a GHE.com host: gh sends those tokens to github.com
// and to *.ghe.com hosts, so with GH_HOST naming a GHE.com tenant they are that tenant's. For
// a GitHub Enterprise Server host gh takes GH_ENTERPRISE_TOKEN instead and GH_TOKEN stays
// github.com's, so such a host, like github.com itself, names no server they belong to. A
// value that cannot be read refuses them, as serverHost's does.
func ghHostServer(value string) (string, bool) {
	host, named := serverHost(value)
	if host == "" {
		return host, named
	}
	if len(host) > len(gheComHostSuffix) &&
		strings.EqualFold(host[len(host)-len(gheComHostSuffix):], gheComHostSuffix) {
		return host, true
	}
	return "", false
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
