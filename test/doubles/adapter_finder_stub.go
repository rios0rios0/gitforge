package doubles

import (
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// AdapterFinderStub implements git.AdapterFinder for testing.
type AdapterFinderStub struct {
	AdapterByServiceTypeValue globalEntities.LocalGitAuthProvider
	AdapterByURLValue         globalEntities.LocalGitAuthProvider
	// AdaptersByURL answers GetAdapterByURL for the URLs it holds, ahead of AdapterByURLValue.
	AdaptersByURL map[string]globalEntities.LocalGitAuthProvider
}

func (s *AdapterFinderStub) GetAdapterByServiceType(
	_ globalEntities.ServiceType,
) globalEntities.LocalGitAuthProvider {
	return s.AdapterByServiceTypeValue
}

func (s *AdapterFinderStub) GetAdapterByURL(url string) globalEntities.LocalGitAuthProvider {
	if adapter, found := s.AdaptersByURL[url]; found {
		return adapter
	}
	return s.AdapterByURLValue
}
