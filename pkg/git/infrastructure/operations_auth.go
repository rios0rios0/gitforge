package infrastructure

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	globalHelpers "github.com/rios0rios0/gitforge/v4/pkg/global/domain/helpers"
)

// GetAuthMethods returns the authentication methods for the given service type.
// It delegates to the appropriate adapter based on the service type.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func (o *GitOperations) GetAuthMethods(
	service globalEntities.ServiceType,
	username string,
) ([]transport.AuthMethod, error) {
	adapter := o.adapterFinder.GetAdapterByServiceType(service)
	if adapter == nil {
		log.Errorf("No authentication mechanism implemented for service type '%v'", service)
		return nil, ErrAuthNotImplemented
	}

	adapter.ConfigureTransport()

	authMethods := adapter.GetAuthMethods(username)

	if len(authMethods) == 0 {
		log.Error("No authentication credentials found for any authentication method")
		return nil, ErrNoAuthMethodFound
	}

	return authMethods, nil
}

// GetRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab), read from
// origin's first URL, or from the first URL of the first remote by name when there is no origin.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func (o *GitOperations) GetRemoteServiceType(repo *git.Repository) (globalEntities.ServiceType, error) {
	cfg, err := repo.Config()
	if err != nil {
		return globalEntities.UNKNOWN, fmt.Errorf("could not get repository config: %w", err)
	}

	firstRemote := serviceRemoteURL(cfg)
	if firstRemote == "" {
		return globalEntities.UNKNOWN, errors.New("no remote URLs configured")
	}

	return o.GetServiceTypeByURL(firstRemote), nil
}

// serviceRemoteURL returns the URL a repository's service type is read from. Origin comes
// first: it is the remote every push goes to, and the credentials consumers build from the
// service type are refused for any push URL off the host of origin's first URL. Remotes are a
// map, iterated in an order that changes from one read to the next, so a repository with
// several remotes used to be given the service type of any of them, and a push to origin the
// credentials of another remote's forge. Without origin, the remotes are read in name order.
func serviceRemoteURL(cfg *config.Config) string {
	if origin, hasOrigin := cfg.Remotes[originRemoteName]; hasOrigin && len(origin.URLs) > 0 {
		return origin.URLs[0]
	}

	for _, name := range slices.Sorted(maps.Keys(cfg.Remotes)) {
		if urls := cfg.Remotes[name].URLs; len(urls) > 0 {
			return urls[0]
		}
	}
	return ""
}

// GetServiceTypeByURL returns the type of the remote service by URL: the service type of the
// adapter whose MatchesURL accepts it, else Bitbucket or AWS CodeCommit, which have no adapter.
// Like the adapters, those match an http:// or https:// URL only on their own hosts
// (globalHelpers.HTTPRemoteServiceType), and any other remote when it contains their name.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func (o *GitOperations) GetServiceTypeByURL(remoteURL string) globalEntities.ServiceType {
	adapter := o.adapterFinder.GetAdapterByURL(remoteURL)
	if adapter != nil {
		return adapter.GetServiceType()
	}

	if serviceType, isHTTP := globalHelpers.HTTPRemoteServiceType(remoteURL); isHTTP {
		if serviceType == globalEntities.BITBUCKET || serviceType == globalEntities.CODECOMMIT {
			return serviceType
		}
		return globalEntities.UNKNOWN
	}
	switch {
	case strings.Contains(remoteURL, "bitbucket.org"):
		return globalEntities.BITBUCKET
	case strings.Contains(remoteURL, "git-codecommit"):
		return globalEntities.CODECOMMIT
	default:
		return globalEntities.UNKNOWN
	}
}
