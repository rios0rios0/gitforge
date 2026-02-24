package infrastructure

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// AdapterFinder provides adapter lookup capabilities without circular dependencies.
type AdapterFinder interface {
	GetAdapterByServiceType(serviceType globalEntities.ServiceType) globalEntities.LocalGitAuthProvider
	GetAdapterByURL(url string) globalEntities.LocalGitAuthProvider
}
