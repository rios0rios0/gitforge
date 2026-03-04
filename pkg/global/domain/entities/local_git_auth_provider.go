package entities

import "github.com/go-git/go-git/v5/plumbing/transport"

// LocalGitAuthProvider extends ForgeProvider with local git authentication.
// This is used by tools that perform local git operations (clone, push) via go-git.
type LocalGitAuthProvider interface {
	ForgeProvider

	// GetServiceType returns the service type identifier for this provider.
	GetServiceType() ServiceType

	// PrepareCloneURL processes the URL before cloning (e.g., stripping embedded credentials).
	PrepareCloneURL(url string) string

	// ConfigureTransport configures any transport-level settings required by this service.
	ConfigureTransport()

	// GetAuthMethods returns the authentication methods for local git operations.
	GetAuthMethods(username string) []transport.AuthMethod
}
