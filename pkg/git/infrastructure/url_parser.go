package infrastructure

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// RemoteURLInfo holds the parsed components of a Git remote URL.
type RemoteURLInfo struct {
	ServiceType  globalEntities.ServiceType
	Organization string
	Project      string // Azure DevOps only; empty for GitHub/GitLab
	RepoName     string
}

// PullRequestURLInfo holds the parsed components of a pull request URL.
type PullRequestURLInfo struct {
	ServiceType  globalEntities.ServiceType
	Organization string
	Project      string // Azure DevOps only; empty for GitHub/GitLab
	RepoName     string
	PRID         int
}

// remoteURLParser defines a function that attempts to parse a cleaned remote URL.
type remoteURLParser func(cleaned string) (*RemoteURLInfo, bool)

// prURLParser defines a function that attempts to parse PR URL path segments.
type prURLParser func(segments []string) (*PullRequestURLInfo, error)

// remoteURLParsers maps URL detection predicates to their parsing functions.
// Each entry is tried in order; the first match wins.
var remoteURLParsers = []struct {
	matches func(string) bool
	parse   remoteURLParser
}{
	{matchesGitHubSSH, parseGitHubSSHRemote},
	{matchesGitHubHTTPS, parseGitHubHTTPSRemote},
	{matchesGitLabSSH, parseGitLabSSHRemote},
	{matchesGitLabHTTPS, parseGitLabHTTPSRemote},
	{matchesAzureDevOpsSSH, parseAzureDevOpsSSHRemote},
	{matchesAzureDevOpsHTTPS, parseAzureDevOpsHTTPSRemote},
}

// prURLParsers maps host substrings to their PR URL parsing functions.
var prURLParsers = map[string]prURLParser{
	"github.com":    parseGitHubPRURL,
	"dev.azure.com": parseAzureDevOpsPRURL,
}

// ParseRemoteURL extracts provider, organization, project, and repository name from a Git remote URL.
// Supported formats:
//   - GitHub SSH:           git@github.com:owner/repo.git
//   - GitHub HTTPS:         https://github.com/owner/repo.git
//   - GitLab SSH:           git@gitlab.com:group/repo.git
//   - GitLab HTTPS:         https://gitlab.com/group/repo.git
//   - Azure DevOps SSH:     git@ssh.dev.azure.com:v3/org/project/repo
//   - Azure DevOps HTTPS:   https://dev.azure.com/org/project/_git/repo
func ParseRemoteURL(rawURL string) (*RemoteURLInfo, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty remote URL")
	}

	cleaned := strings.TrimSuffix(rawURL, ".git")

	for _, entry := range remoteURLParsers {
		if entry.matches(cleaned) {
			info, ok := entry.parse(cleaned)
			if ok {
				return info, nil
			}
		}
	}

	return nil, fmt.Errorf("unsupported remote URL format: %s", rawURL)
}

// ParsePullRequestURL extracts provider, organization, project, repository, and PR ID from a PR URL.
// Supported formats:
//   - GitHub:        https://github.com/{org}/{repo}/pull/{id}
//   - Azure DevOps:  https://dev.azure.com/{org}/{project}/_git/{repo}/pullrequest/{id}
func ParsePullRequestURL(rawURL string) (*PullRequestURLInfo, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty pull request URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Host)
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	for hostKey, parser := range prURLParsers {
		if strings.Contains(host, hostKey) {
			return parser(segments)
		}
	}

	return nil, fmt.Errorf("unsupported provider host: %q", host)
}

// --- GitHub remote URL matchers and parsers ---

func matchesGitHubSSH(cleaned string) bool {
	return strings.HasPrefix(cleaned, "git@github.com:")
}

func parseGitHubSSHRemote(cleaned string) (*RemoteURLInfo, bool) {
	path := strings.TrimPrefix(cleaned, "git@github.com:")
	parts := strings.Split(path, "/")
	if len(parts) < 2 { //nolint:mnd // owner/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.GITHUB,
		Organization: parts[0],
		RepoName:     parts[1],
	}, true
}

func matchesGitHubHTTPS(cleaned string) bool {
	return strings.Contains(cleaned, "github.com/")
}

func parseGitHubHTTPSRemote(cleaned string) (*RemoteURLInfo, bool) {
	_, after, _ := strings.Cut(cleaned, "github.com/")
	parts := strings.Split(after, "/")
	if len(parts) < 2 { //nolint:mnd // owner/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.GITHUB,
		Organization: parts[0],
		RepoName:     parts[1],
	}, true
}

// --- GitLab remote URL matchers and parsers ---

func matchesGitLabSSH(cleaned string) bool {
	return strings.HasPrefix(cleaned, "git@") && strings.Contains(cleaned, "gitlab")
}

func parseGitLabSSHRemote(cleaned string) (*RemoteURLInfo, bool) {
	colonIdx := strings.Index(cleaned, ":")
	if colonIdx < 0 {
		return nil, false
	}
	path := cleaned[colonIdx+1:]
	parts := strings.Split(path, "/")
	if len(parts) < 2 { //nolint:mnd // group/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.GITLAB,
		Organization: strings.Join(parts[:len(parts)-1], "/"),
		RepoName:     parts[len(parts)-1],
	}, true
}

func matchesGitLabHTTPS(cleaned string) bool {
	return strings.Contains(cleaned, "gitlab.com/")
}

func parseGitLabHTTPSRemote(cleaned string) (*RemoteURLInfo, bool) {
	_, after, _ := strings.Cut(cleaned, "gitlab.com/")
	parts := strings.Split(after, "/")
	if len(parts) < 2 { //nolint:mnd // group/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.GITLAB,
		Organization: strings.Join(parts[:len(parts)-1], "/"),
		RepoName:     parts[len(parts)-1],
	}, true
}

// --- Azure DevOps remote URL matchers and parsers ---

func matchesAzureDevOpsSSH(cleaned string) bool {
	return strings.HasPrefix(cleaned, "git@") && strings.Contains(cleaned, "dev.azure.com")
}

func parseAzureDevOpsSSHRemote(cleaned string) (*RemoteURLInfo, bool) {
	parts := strings.Split(cleaned, "/")
	if len(parts) < 4 { //nolint:mnd // v3/org/project/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.AZUREDEVOPS,
		Organization: parts[1],
		Project:      parts[2],
		RepoName:     parts[3],
	}, true
}

func matchesAzureDevOpsHTTPS(cleaned string) bool {
	return strings.Contains(cleaned, "dev.azure.com")
}

func parseAzureDevOpsHTTPSRemote(cleaned string) (*RemoteURLInfo, bool) {
	parts := strings.Split(cleaned, "/")
	if len(parts) < 7 { //nolint:mnd // https://dev.azure.com/org/project/_git/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.AZUREDEVOPS,
		Organization: parts[3],
		Project:      parts[4],
		RepoName:     parts[6],
	}, true
}

// --- Pull request URL parsers ---

func parseGitHubPRURL(segments []string) (*PullRequestURLInfo, error) {
	// Expected: {org}/{repo}/pull/{id}
	if len(segments) < 4 || segments[2] != "pull" { //nolint:mnd // org/repo/pull/id
		return nil, fmt.Errorf("invalid GitHub PR URL format, expected: /{org}/{repo}/pull/{id}")
	}

	prID, err := strconv.Atoi(segments[3])
	if err != nil {
		return nil, fmt.Errorf("invalid PR ID %q: %w", segments[3], err)
	}

	return &PullRequestURLInfo{
		ServiceType:  globalEntities.GITHUB,
		Organization: segments[0],
		RepoName:     segments[1],
		PRID:         prID,
	}, nil
}

func parseAzureDevOpsPRURL(segments []string) (*PullRequestURLInfo, error) {
	// Expected: {org}/{project}/_git/{repo}/pullrequest/{id}
	if len(segments) < 6 || segments[2] != "_git" || segments[4] != "pullrequest" { //nolint:mnd // full path
		return nil, fmt.Errorf(
			"invalid Azure DevOps PR URL format, expected: /{org}/{project}/_git/{repo}/pullrequest/{id}",
		)
	}

	prID, err := strconv.Atoi(segments[5])
	if err != nil {
		return nil, fmt.Errorf("invalid PR ID %q: %w", segments[5], err)
	}

	return &PullRequestURLInfo{
		ServiceType:  globalEntities.AZUREDEVOPS,
		Organization: segments[0],
		Project:      segments[1],
		RepoName:     segments[3],
		PRID:         prID,
	}, nil
}
