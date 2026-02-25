package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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

// GetRemoteServiceType returns the type of the remote service (e.g. GitHub, GitLab).
// Exported for use by autobump (github.com/rios0rios0/autobump).
func (o *GitOperations) GetRemoteServiceType(repo *git.Repository) (globalEntities.ServiceType, error) {
	cfg, err := repo.Config()
	if err != nil {
		return globalEntities.UNKNOWN, fmt.Errorf("could not get repository config: %w", err)
	}

	var firstRemote string
	for _, remote := range cfg.Remotes {
		if len(remote.URLs) == 0 {
			continue
		}
		firstRemote = remote.URLs[0]
		break
	}

	if firstRemote == "" {
		return globalEntities.UNKNOWN, errors.New("no remote URLs configured")
	}

	return o.GetServiceTypeByURL(firstRemote), nil
}

// GetServiceTypeByURL returns the type of the remote service by URL.
// Exported for use by autobump (github.com/rios0rios0/autobump).
func (o *GitOperations) GetServiceTypeByURL(remoteURL string) globalEntities.ServiceType {
	adapter := o.adapterFinder.GetAdapterByURL(remoteURL)
	if adapter != nil {
		return adapter.GetServiceType()
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
