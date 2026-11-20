package helpers

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// visualStudioHostSuffix ends the host of an Azure DevOps organization's legacy address,
// https://<org>.visualstudio.com.
const visualStudioHostSuffix = ".visualstudio.com"

// forgeHTTPHosts maps the host of an HTTP(S) remote on a forge to the forge. Two kinds of host
// are recognised by their shape instead (HTTPRemoteServiceType): an Azure DevOps organization's
// <org>.visualstudio.com and AWS CodeCommit's regional hosts.
//
//nolint:gochecknoglobals // read-only lookup table
var forgeHTTPHosts = map[string]entities.ServiceType{
	"github.com":        entities.GITHUB,
	"www.github.com":    entities.GITHUB,
	"gitlab.com":        entities.GITLAB,
	"www.gitlab.com":    entities.GITLAB,
	"dev.azure.com":     entities.AZUREDEVOPS,
	"codeberg.org":      entities.CODEBERG,
	"bitbucket.org":     entities.BITBUCKET,
	"www.bitbucket.org": entities.BITBUCKET,
}

// codeCommitHostPattern matches the host of an AWS CodeCommit HTTPS remote:
// git-codecommit.<region>.amazonaws.com, its FIPS endpoint git-codecommit-fips.<region>..., and
// the China regions' amazonaws.com.cn.
var codeCommitHostPattern = regexp.MustCompile(`^git-codecommit(-fips)?\.[a-z0-9-]+\.amazonaws\.com(\.cn)?$`)

// HTTPURLHost returns the host of an http:// or https:// remote URL, in lower case and without
// its user information or port, and true. An HTTP(S) URL whose host cannot be read gives "" and
// true, which is no forge's host, and any other remote, such as a scp-like git@host:path, an
// ssh:// URL or a local path, gives "" and false.
//
// A provider compares this host with its forge's own hosts before it takes an HTTP(S) remote for
// the forge, because the token it holds travels to that host: a URL that merely contains the
// forge's name, such as https://github.com.example.net/..., is another server. An SSH remote is
// left to the provider's looser matching, since SSH authenticates with keys and its host aliases
// (github.com-work) are resolved by ssh_config.
func HTTPURLHost(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		scheme, _, _ := strings.Cut(trimmed, ":")
		return "", isHTTPScheme(scheme)
	}
	if !isHTTPScheme(parsed.Scheme) {
		return "", false
	}
	return strings.ToLower(parsed.Hostname()), true
}

// HTTPRemoteServiceType returns the forge an http:// or https:// remote URL is on, judged by its
// host (HTTPURLHost) alone, and true: GITHUB for github.com and www.github.com, GITLAB for
// gitlab.com and www.gitlab.com, AZUREDEVOPS for dev.azure.com and <org>.visualstudio.com,
// CODEBERG for codeberg.org, BITBUCKET for bitbucket.org and www.bitbucket.org, CODECOMMIT for
// git-codecommit[-fips].<region>.amazonaws.com[.cn], and UNKNOWN for any other host, such as one
// that merely contains a forge's name or cannot be read. For a remote that is no HTTP(S) URL it
// returns UNKNOWN and false, and the caller matches it its own way. Every place gitforge tells a
// forge from an HTTP(S) URL asks this, so they all tell it the same way.
func HTTPRemoteServiceType(rawURL string) (entities.ServiceType, bool) {
	host, isHTTP := HTTPURLHost(rawURL)
	if !isHTTP {
		return entities.UNKNOWN, false
	}
	if serviceType, isForgeHost := forgeHTTPHosts[host]; isForgeHost {
		return serviceType, true
	}
	if _, isLegacy := VisualStudioOrganization(host); isLegacy {
		return entities.AZUREDEVOPS, true
	}
	if codeCommitHostPattern.MatchString(host) {
		return entities.CODECOMMIT, true
	}
	return entities.UNKNOWN, true
}

// VisualStudioOrganization returns the organization a legacy Azure DevOps host names,
// <org>.visualstudio.com, as it is written, and whether host is one: a single label before
// visualstudio.com, whose letter case does not matter.
func VisualStudioOrganization(host string) (string, bool) {
	if len(host) <= len(visualStudioHostSuffix) ||
		!strings.EqualFold(host[len(host)-len(visualStudioHostSuffix):], visualStudioHostSuffix) {
		return "", false
	}
	organization := host[:len(host)-len(visualStudioHostSuffix)]
	if strings.Contains(organization, ".") {
		return "", false
	}
	return organization, true
}

// isHTTPScheme reports whether scheme is http or https, whatever its letter case.
func isHTTPScheme(scheme string) bool {
	return strings.EqualFold(scheme, "https") || strings.EqualFold(scheme, "http")
}
