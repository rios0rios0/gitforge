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

func extractOrgName(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return u.Host
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
