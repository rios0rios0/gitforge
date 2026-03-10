package infrastructure

import (
	"fmt"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// ProviderRegistry manages all registered Git provider implementations.
// It supports both factory-based creation (by name + token) and direct adapter lookup.
type ProviderRegistry struct {
	factories   map[string]ProviderFactory
	adapters    []globalEntities.ForgeProvider
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
func (r *ProviderRegistry) RegisterAdapter(adapter globalEntities.ForgeProvider) {
	r.adapters = append(r.adapters, adapter)
}

// RegisterDiscoverer adds a discoverer factory under the given provider name.
func (r *ProviderRegistry) RegisterDiscoverer(name string, factory DiscovererFactory) {
	r.discoverers[name] = factory
}

// Get returns a configured provider instance for the given name and token.
func (r *ProviderRegistry) Get(name, token string) (globalEntities.ForgeProvider, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %q", name)
	}
	return factory(token), nil
}

// GetDiscoverer returns a configured discoverer instance for the given provider name and token.
func (r *ProviderRegistry) GetDiscoverer(name, token string) (globalEntities.RepositoryDiscoverer, error) {
	factory, ok := r.discoverers[name]
	if !ok {
		return nil, fmt.Errorf("unknown discoverer type: %q", name)
	}
	return factory(token), nil
}

// GetAdapterByURL returns the appropriate adapter for the given URL.
func (r *ProviderRegistry) GetAdapterByURL(url string) globalEntities.ForgeProvider {
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
	serviceType globalEntities.ServiceType,
) globalEntities.LocalGitAuthProvider {
	for _, adapter := range r.adapters {
		if lgap, ok := adapter.(globalEntities.LocalGitAuthProvider); ok {
			if lgap.GetServiceType() == serviceType {
				return lgap
			}
		}
	}
	return nil
}

// GetReviewProvider returns a configured ReviewProvider instance for the given name and token.
// It returns an error if the provider does not implement ReviewProvider.
func (r *ProviderRegistry) GetReviewProvider(
	name, token string,
) (globalEntities.ReviewProvider, error) {
	provider, err := r.Get(name, token)
	if err != nil {
		return nil, err
	}

	reviewProvider, ok := provider.(globalEntities.ReviewProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not implement ReviewProvider", name)
	}

	return reviewProvider, nil
}

// ServiceTypeToProviderName maps a ServiceType to the provider name string
// used for registry lookups. Returns empty string for unknown service types.
//
// Exported for use by autobump (github.com/rios0rios0/autobump) and
// autoupdate (github.com/rios0rios0/autoupdate).
func ServiceTypeToProviderName(serviceType globalEntities.ServiceType) string {
	providerNames := map[globalEntities.ServiceType]string{
		globalEntities.GITHUB:      "github",
		globalEntities.GITLAB:      "gitlab",
		globalEntities.AZUREDEVOPS: "azuredevops",
	}
	return providerNames[serviceType]
}

// Names returns the list of registered provider factory names.
func (r *ProviderRegistry) Names() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}
