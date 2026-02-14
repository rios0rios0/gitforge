package registry

import (
	"fmt"

	"github.com/rios0rios0/gitforge/domain/entities"
	domainRepos "github.com/rios0rios0/gitforge/domain/repositories"
)

// ProviderFactory is a constructor function that creates a ForgeProvider given an auth token.
type ProviderFactory func(token string) domainRepos.ForgeProvider

// DiscovererFactory is a constructor that creates a RepositoryDiscoverer given an auth token.
type DiscovererFactory func(token string) entities.RepositoryDiscoverer

// ProviderRegistry manages all registered Git provider implementations.
// It supports both factory-based creation (by name + token) and direct adapter lookup.
type ProviderRegistry struct {
	factories   map[string]ProviderFactory
	adapters    []domainRepos.ForgeProvider
	discoverers map[string]DiscovererFactory
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		factories:   make(map[string]ProviderFactory),
		discoverers: make(map[string]DiscovererFactory),
	}
}

// RegisterFactory adds a provider factory under the given name (e.g. "github").
func (r *ProviderRegistry) RegisterFactory(name string, factory ProviderFactory) {
	r.factories[name] = factory
}

// RegisterAdapter adds a pre-created provider adapter for URL and service type lookups.
func (r *ProviderRegistry) RegisterAdapter(adapter domainRepos.ForgeProvider) {
	r.adapters = append(r.adapters, adapter)
}

// RegisterDiscoverer adds a discoverer factory under the given provider name.
func (r *ProviderRegistry) RegisterDiscoverer(name string, factory DiscovererFactory) {
	r.discoverers[name] = factory
}

// Get returns a configured provider instance for the given name and token.
func (r *ProviderRegistry) Get(name, token string) (domainRepos.ForgeProvider, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %q", name)
	}
	return factory(token), nil
}

// GetDiscoverer returns a configured discoverer instance for the given provider name and token.
func (r *ProviderRegistry) GetDiscoverer(name, token string) (entities.RepositoryDiscoverer, error) {
	factory, ok := r.discoverers[name]
	if !ok {
		return nil, fmt.Errorf("unknown discoverer type: %q", name)
	}
	return factory(token), nil
}

// GetAdapterByURL returns the appropriate adapter for the given URL.
func (r *ProviderRegistry) GetAdapterByURL(url string) domainRepos.ForgeProvider {
	for _, adapter := range r.adapters {
		if adapter.MatchesURL(url) {
			return adapter
		}
	}
	return nil
}

// GetAdapterByServiceType returns the adapter for the given service type.
// Only works with adapters that implement LocalGitAuthProvider.
func (r *ProviderRegistry) GetAdapterByServiceType(
	serviceType entities.ServiceType,
) domainRepos.LocalGitAuthProvider {
	for _, adapter := range r.adapters {
		if lgap, ok := adapter.(domainRepos.LocalGitAuthProvider); ok {
			if lgap.GetServiceType() == serviceType {
				return lgap
			}
		}
	}
	return nil
}

// Names returns the list of registered provider factory names.
func (r *ProviderRegistry) Names() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}
