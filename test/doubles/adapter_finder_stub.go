package doubles

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// AdapterFinderStub implements git.AdapterFinder for testing.
type AdapterFinderStub struct {
	AdapterByServiceTypeValue globalEntities.LocalGitAuthProvider
	AdapterByURLValue         globalEntities.LocalGitAuthProvider
}

func (s *AdapterFinderStub) GetAdapterByServiceType(
	_ globalEntities.ServiceType,
) globalEntities.LocalGitAuthProvider {
	return s.AdapterByServiceTypeValue
}

func (s *AdapterFinderStub) GetAdapterByURL(_ string) globalEntities.LocalGitAuthProvider {
	return s.AdapterByURLValue
}
