package infrastructure

import (
	"os"
	"strings"
)

// gitlabCIServerVariables are the variables a GitLab CI job is given that name the GitLab
// server it runs on, the server that issued its CI_JOB_TOKEN. Every GitLab CI job sets them.
//
//nolint:gochecknoglobals // read-only lookup table
var gitlabCIServerVariables = []string{"CI_SERVER_URL", "CI_SERVER_HOST", "CI_SERVER_FQDN", "CI_API_V4_URL"}

// GitLabCIServerHost returns the host of the GitLab server a GitLab CI job runs on, read from
// CI_SERVER_URL, CI_SERVER_HOST, CI_SERVER_FQDN and CI_API_V4_URL: that server issued the job's
// CI_JOB_TOKEN and is the only one that accepts it. Other variables that name a GitLab server,
// such as GITLAB_HOST, describe other tokens and are not read. It fails, with an error matching
// entities.ErrNoCredential, when none of the four is set (the run is no GitLab CI job), when one
// cannot be read, or when two name different hosts. A nil lookup reads the process
// environment.
func GitLabCIServerHost(lookup func(string) string) (string, error) {
	if lookup == nil {
		lookup = os.Getenv
	}
	found, foundIn := "", ""
	for _, name := range gitlabCIServerVariables {
		value := strings.TrimSpace(lookup(name))
		if value == "" {
			continue
		}
		host, _ := serverHost(value)
		if host == "" {
			return "", newUnavailableError(nil, "%s cannot be read as the GitLab server of the CI job", name)
		}
		if found == "" {
			found, foundIn = host, name
			continue
		}
		if !strings.EqualFold(host, found) {
			return "", newUnavailableError(
				nil, "%s names the GitLab server %s but %s names %s", foundIn, found, name, host,
			)
		}
	}
	if found == "" {
		return "", newUnavailableError(
			nil, "none of %s names the GitLab server of a CI job", strings.Join(gitlabCIServerVariables, ", "),
		)
	}
	return found, nil
}

// CheckGitLabCIJobTokenServer returns nil when a GitLab CI job's CI_JOB_TOKEN may be sent to
// gitlab.com: GitLabCIServerHost says the job runs there. Otherwise it returns an error
// matching entities.ErrNoCredential that says why not: the job runs on another server, whose
// token gitlab.com must not receive, or no CI variable names the server at all. A nil lookup
// reads the process environment.
func CheckGitLabCIJobTokenServer(lookup func(string) string) error {
	host, err := GitLabCIServerHost(lookup)
	if err != nil {
		return newUnavailableError(err, "CI_JOB_TOKEN is not sent to %s", gitlabHost)
	}
	if !strings.EqualFold(host, gitlabHost) {
		return newUnavailableError(
			nil, "the CI job runs on the GitLab server %s, so CI_JOB_TOKEN is not sent to %s", host, gitlabHost,
		)
	}
	return nil
}
