package infrastructure

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	globalHelpers "github.com/rios0rios0/gitforge/v4/pkg/global/domain/helpers"
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

// httpRemoteParser reads the path segments of an http(s) remote on a forge's own host, which it
// is given as written.
type httpRemoteParser func(hostname string, segments []string) (*RemoteURLInfo, bool)

// prURLParser defines a function that attempts to parse the path segments of a PR URL on a
// forge's own host, which it is given as written.
type prURLParser func(hostname string, segments []string) (*PullRequestURLInfo, error)

// remoteURLParserEntry pairs a URL detection predicate with its parsing function.
type remoteURLParserEntry struct {
	matches func(string) bool
	parse   remoteURLParser
}

// getRemoteURLParsers returns the ordered list of parsers for a remote that is no http(s) URL.
// Each entry is tried in order; the first match wins. They match a substring of the forge's
// name, as they always have: the SSH ones take host aliases such as github.com-work, which
// ssh_config resolves, and the ones named HTTPS now see only the remaining forms, git://,
// file://, scheme-less remotes and local paths. An http(s) URL is read by its host instead
// (getHTTPRemoteParsers).
func getRemoteURLParsers() []remoteURLParserEntry {
	return []remoteURLParserEntry{
		{matchesGitHubSSH, parseGitHubSSHRemote},
		{matchesGitHubHTTPS, parseGitHubHTTPSRemote},
		{matchesGitLabSSH, parseGitLabSSHRemote},
		{matchesGitLabHTTPS, parseGitLabHTTPSRemote},
		{matchesAzureDevOpsSSH, parseAzureDevOpsSSHRemote},
		{matchesAzureDevOpsHTTPS, parseAzureDevOpsHTTPSRemote},
	}
}

// getHTTPRemoteParsers maps a forge to the parser of an http(s) remote on one of its own hosts,
// the forge globalHelpers.HTTPRemoteServiceType tells from the URL's host. A forge without one
// has no HTTP(S) remote ParseRemoteURL reads.
func getHTTPRemoteParsers() map[globalEntities.ServiceType]httpRemoteParser {
	return map[globalEntities.ServiceType]httpRemoteParser{
		globalEntities.UNKNOWN:     nil,
		globalEntities.GITHUB:      parseGitHubHTTPSPath,
		globalEntities.GITLAB:      parseGitLabHTTPSPath,
		globalEntities.AZUREDEVOPS: parseAzureDevOpsHTTPSPath,
		globalEntities.BITBUCKET:   nil,
		globalEntities.CODECOMMIT:  nil,
		globalEntities.CODEBERG:    nil,
	}
}

// getPRURLParsers maps a forge to the parser of a pull request URL on one of its own hosts, the
// forge globalHelpers.HTTPRemoteServiceType tells from the URL's host. A forge without one has
// no pull request URL ParsePullRequestURL reads.
func getPRURLParsers() map[globalEntities.ServiceType]prURLParser {
	return map[globalEntities.ServiceType]prURLParser{
		globalEntities.UNKNOWN:     nil,
		globalEntities.GITHUB:      parseGitHubPRURL,
		globalEntities.GITLAB:      nil,
		globalEntities.AZUREDEVOPS: parseAzureDevOpsPRURL,
		globalEntities.BITBUCKET:   nil,
		globalEntities.CODECOMMIT:  nil,
		globalEntities.CODEBERG:    nil,
	}
}

// ParseRemoteURL extracts provider, organization, project, and repository name from a Git remote URL.
// Supported formats:
//   - GitHub SSH:           git@github.com:owner/repo.git
//   - GitHub HTTPS:         https://github.com/owner/repo.git
//   - GitLab SSH:           git@gitlab.com:group/repo.git
//   - GitLab HTTPS:         https://gitlab.com/group/repo.git
//   - Azure DevOps SSH:     git@ssh.dev.azure.com:v3/org/project/repo
//   - Azure DevOps HTTPS:   https://dev.azure.com/org/project/_git/repo
//   - Azure DevOps HTTPS:   https://org.visualstudio.com/[collection/]project/_git/repo
//
// An http:// or https:// URL is read only on a forge's own host, the one
// globalHelpers.HTTPRemoteServiceType names, as the providers' MatchesURL reads it:
// https://github.com.example.net/owner/repo, https://notgithub.com/owner/repo and a GitHub
// Enterprise host such as https://github.company.com/owner/repo are unsupported. Any other
// remote is matched by a substring of the forge's name, as before.
//
// Each SSH format may also be written as a URL, ssh://[user@]host[:port]/path, which parses
// exactly like the scp-like form above it: ssh://git@ssh.dev.azure.com:22/v3/org/project/repo
// names the same repository as git@ssh.dev.azure.com:v3/org/project/repo.
func ParseRemoteURL(rawURL string) (*RemoteURLInfo, error) {
	if rawURL == "" {
		return nil, errors.New("empty remote URL")
	}

	cleaned := asSCPLikeRemote(strings.TrimSuffix(rawURL, ".git"))
	if serviceType, isHTTP := globalHelpers.HTTPRemoteServiceType(cleaned); isHTTP {
		if info, ok := parseHTTPRemote(serviceType, cleaned); ok {
			return info, nil
		}
		return nil, fmt.Errorf("unsupported remote URL format: %s", withoutUserinfo(rawURL))
	}

	for _, entry := range getRemoteURLParsers() {
		if entry.matches(cleaned) {
			info, ok := entry.parse(cleaned)
			if ok {
				return info, nil
			}
		}
	}

	return nil, fmt.Errorf("unsupported remote URL format: %s", withoutUserinfo(rawURL))
}

// ParsePullRequestURL extracts provider, organization, project, repository, and PR ID from a PR URL.
// Supported formats:
//   - GitHub:        https://github.com/{org}/{repo}/pull/{id}
//   - Azure DevOps:  https://dev.azure.com/{org}/{project}/_git/{repo}/pullrequest/{id}
//   - Azure DevOps:  https://{org}.visualstudio.com/[{collection}/]{project}/_git/{repo}/pullrequest/{id}
//
// The forge is told by the URL's host alone, as globalHelpers.HTTPRemoteServiceType tells it, so
// a host that merely contains a forge's name, such as github.com.example.net, is unsupported.
func ParsePullRequestURL(rawURL string) (*PullRequestURLInfo, error) {
	if rawURL == "" {
		return nil, errors.New("empty pull request URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Host)
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	serviceType, _ := globalHelpers.HTTPRemoteServiceType(rawURL)
	if parser := getPRURLParsers()[serviceType]; parser != nil {
		return parser(parsed.Hostname(), segments)
	}

	return nil, fmt.Errorf("unsupported provider host: %q", host)
}

// parseHTTPRemote reads an http(s) remote on a host of the given forge, with the parser
// getHTTPRemoteParsers holds for it.
func parseHTTPRemote(serviceType globalEntities.ServiceType, cleaned string) (*RemoteURLInfo, bool) {
	parser := getHTTPRemoteParsers()[serviceType]
	if parser == nil {
		return nil, false
	}
	parsed, err := url.Parse(strings.TrimSpace(cleaned))
	if err != nil {
		return nil, false
	}
	// The escaped path keeps a segment as the remote writes it, as the scp-like form does.
	return parser(parsed.Hostname(), strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/"))
}

// withoutUserinfo returns rawURL for an error message, without the user information a URL can
// carry before its host, which may be a token. A scp-like remote is returned as it is.
func withoutUserinfo(rawURL string) string {
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return rawURL
	}
	authorityStart := schemeEnd + len("://")
	authorityEnd := len(rawURL)
	if end := strings.IndexAny(rawURL[authorityStart:], "/?#"); end >= 0 {
		authorityEnd = authorityStart + end
	}
	if at := strings.LastIndex(rawURL[authorityStart:authorityEnd], "@"); at >= 0 {
		return rawURL[:authorityStart] + rawURL[authorityStart+at+1:]
	}
	return rawURL
}

// asSCPLikeRemote rewrites an SSH remote written as a URL, ssh://[user@]host[:port]/path, as the
// scp-like git@host:path the SSH parsers read, and returns any other remote unchanged. Read as
// it was written, such a URL reached the HTTPS parsers, which took Azure DevOps's v3 segment for
// the organization and rejected a host followed by a port. The user and the port choose the
// account and the port to connect with, not the repository, so neither carries over, and the
// path keeps its escaping, as the scp-like form does.
func asSCPLikeRemote(cleaned string) string {
	parsed, err := url.Parse(cleaned)
	if err != nil || parsed.Scheme != sshProtocol || parsed.Hostname() == "" {
		return cleaned
	}
	return "git@" + parsed.Hostname() + ":" + strings.TrimPrefix(parsed.EscapedPath(), "/")
}

// --- GitHub remote URL matchers and parsers ---

func matchesGitHubSSH(cleaned string) bool {
	return strings.HasPrefix(cleaned, "git@") && strings.Contains(cleaned, "github.com")
}

func parseGitHubSSHRemote(cleaned string) (*RemoteURLInfo, bool) {
	_, after, ok := strings.Cut(cleaned, ":")
	if !ok {
		return nil, false
	}
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

// parseGitHubHTTPSPath reads owner/repo from the path of an http(s) remote on github.com.
func parseGitHubHTTPSPath(_ string, segments []string) (*RemoteURLInfo, bool) {
	if len(segments) < 2 { //nolint:mnd // owner/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.GITHUB,
		Organization: segments[0],
		RepoName:     segments[1],
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
	_, after, ok := strings.Cut(cleaned, ":")
	if !ok {
		return nil, false
	}
	path := after
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

// parseGitLabHTTPSPath reads group/[subgroup/...]repo from the path of an http(s) remote on
// gitlab.com.
func parseGitLabHTTPSPath(_ string, segments []string) (*RemoteURLInfo, bool) {
	if len(segments) < 2 { //nolint:mnd // group/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.GITLAB,
		Organization: strings.Join(segments[:len(segments)-1], "/"),
		RepoName:     segments[len(segments)-1],
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

// parseAzureDevOpsHTTPSPath reads an http(s) remote on Azure DevOps: org/project/_git/repo on
// dev.azure.com, and [collection/]project/_git/repo on an organization's legacy
// <org>.visualstudio.com, whose organization is in the host.
func parseAzureDevOpsHTTPSPath(hostname string, segments []string) (*RemoteURLInfo, bool) {
	if organization, isLegacy := globalHelpers.VisualStudioOrganization(hostname); isLegacy {
		gitSegment := slices.Index(segments, "_git")
		if gitSegment < 1 || gitSegment+1 >= len(segments) {
			return nil, false
		}
		return &RemoteURLInfo{
			ServiceType:  globalEntities.AZUREDEVOPS,
			Organization: organization,
			Project:      segments[gitSegment-1],
			RepoName:     segments[gitSegment+1],
		}, true
	}
	if len(segments) < 4 { //nolint:mnd // org/project/_git/repo
		return nil, false
	}
	return &RemoteURLInfo{
		ServiceType:  globalEntities.AZUREDEVOPS,
		Organization: segments[0],
		Project:      segments[1],
		RepoName:     segments[3],
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

func parseGitHubPRURL(_ string, segments []string) (*PullRequestURLInfo, error) {
	// Expected: {org}/{repo}/pull/{id}
	if len(segments) < 4 || segments[2] != "pull" {
		return nil, errors.New("invalid GitHub PR URL format, expected: /{org}/{repo}/pull/{id}")
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

func parseAzureDevOpsPRURL(hostname string, segments []string) (*PullRequestURLInfo, error) {
	if organization, isLegacy := globalHelpers.VisualStudioOrganization(hostname); isLegacy {
		return parseVisualStudioPRURL(organization, segments)
	}

	// Expected: {org}/{project}/_git/{repo}/pullrequest/{id}
	if len(segments) < 6 || segments[2] != "_git" || segments[4] != "pullrequest" {
		return nil, errors.New(
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

// parseVisualStudioPRURL reads a pull request URL on an organization's legacy
// <org>.visualstudio.com, whose path may begin with a collection.
func parseVisualStudioPRURL(organization string, segments []string) (*PullRequestURLInfo, error) {
	// Expected: [{collection}/]{project}/_git/{repo}/pullrequest/{id}
	gitSegment := slices.Index(segments, "_git")
	if gitSegment < 1 || len(segments) < gitSegment+4 || segments[gitSegment+2] != "pullrequest" {
		return nil, errors.New(
			"invalid Azure DevOps PR URL format, expected: /[{collection}/]{project}/_git/{repo}/pullrequest/{id}",
		)
	}

	prID, err := strconv.Atoi(segments[gitSegment+3])
	if err != nil {
		return nil, fmt.Errorf("invalid PR ID %q: %w", segments[gitSegment+3], err)
	}

	return &PullRequestURLInfo{
		ServiceType:  globalEntities.AZUREDEVOPS,
		Organization: organization,
		Project:      segments[gitSegment-1],
		RepoName:     segments[gitSegment+1],
		PRID:         prID,
	}, nil
}
