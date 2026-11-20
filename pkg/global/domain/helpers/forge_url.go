package helpers

import (
	"net/url"
	"strings"
)

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

// isHTTPScheme reports whether scheme is http or https, whatever its letter case.
func isHTTPScheme(scheme string) bool {
	return strings.EqualFold(scheme, "https") || strings.EqualFold(scheme, "http")
}
