package doubles

import (
	"context"
	"slices"
	"sync"
)

// AzureTenantResolverStub is a recording credentials AzureTenantResolver that answers
// every organization with the same tenant or error, without network access.
type AzureTenantResolverStub struct {
	Tenant string
	Err    error

	mu            sync.Mutex
	organizations []string
}

// GetTenant records the organization and returns the configured tenant or error.
func (s *AzureTenantResolverStub) GetTenant(_ context.Context, organization string) (string, error) {
	s.mu.Lock()
	s.organizations = append(s.organizations, organization)
	s.mu.Unlock()
	return s.Tenant, s.Err
}

// Organizations returns every organization GetTenant was asked about.
func (s *AzureTenantResolverStub) Organizations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.organizations)
}
