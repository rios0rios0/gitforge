package azuredevops

import (
	"net/url"
	"strings"
)

func normalizeOrgURL(org string) string {
	org = strings.TrimSuffix(org, "/")
	if !strings.HasPrefix(org, "https://") {
		org = "https://dev.azure.com/" + org
	}
	return org
}

func buildBaseURL(orgName string) string {
	if orgName == "" {
		return "https://dev.azure.com"
	}
	return "https://dev.azure.com/" + strings.Split(orgName, "/")[0]
}

// visualStudioHostSuffix ends the host of an organization's legacy Azure DevOps address,
// https://<org>.visualstudio.com.
const visualStudioHostSuffix = ".visualstudio.com"

// extractOrgName returns the organization an Azure DevOps URL names: the first path segment of
// https://dev.azure.com/<org>/..., the first label of the host of the legacy
// https://<org>.visualstudio.com/..., and the segment after v3 of an SSH remote,
// git@ssh.dev.azure.com:v3/<org>/<project>/<repo> or
// <org>@vs-ssh.visualstudio.com:v3/<org>/<project>/<repo>, written scp-like or as an ssh://
// URL. A value that does not parse as a URL is returned as it is.
func extractOrgName(baseURL string) string {
	if path, isSSH := sshRemotePath(baseURL); isSSH {
		segments := strings.Split(strings.Trim(path, "/"), "/")
		if len(segments) > 1 && segments[0] == "v3" {
			return segments[1]
		}
		return segments[0]
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	if organization, isLegacy := visualStudioOrganization(u.Hostname()); isLegacy {
		return organization
	}
	return strings.Split(strings.Trim(u.Path, "/"), "/")[0]
}

// sshRemotePath returns the path of an SSH remote, written as an ssh:// URL or in git's
// scp-like [user@]host:path form, and whether rawURL is one.
func sshRemotePath(rawURL string) (string, bool) {
	if scheme, _, hasScheme := strings.Cut(rawURL, "://"); hasScheme {
		if !strings.EqualFold(scheme, "ssh") {
			return "", false
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", false
		}
		return u.Path, true
	}
	address, path, hasColon := strings.Cut(rawURL, ":")
	if !hasColon || address == "" || strings.Contains(address, "/") {
		return "", false
	}
	return path, true
}

// visualStudioOrganization returns the organization a legacy Azure DevOps host names,
// <org>.visualstudio.com, and whether host is one: a single label before visualstudio.com,
// whose letter case does not matter.
func visualStudioOrganization(host string) (string, bool) {
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

// stripUsernameFromURL removes the username from a URL if present.
func stripUsernameFromURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return rawURL
	}
	const skipping = "://"
	schemeEnd := strings.Index(rawURL, skipping) + len(skipping)
	atIndex := strings.Index(rawURL[schemeEnd:], "@")
	if atIndex == -1 {
		return rawURL
	}
	return rawURL[:schemeEnd] + rawURL[schemeEnd+atIndex+1:]
}
