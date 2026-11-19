package doubles

import (
	"context"
	"slices"
	"sync"

	credentialEntities "github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
)

// CredentialResolverStub is a recording credentials CredentialResolver that answers every
// request with the same credential or error.
type CredentialResolverStub struct {
	Credential credentialEntities.Credential
	Err        error

	mu       sync.Mutex
	requests []credentialEntities.CredentialRequest
}

// Resolve records the request and returns the configured credential or error.
func (s *CredentialResolverStub) Resolve(
	_ context.Context, request credentialEntities.CredentialRequest,
) (credentialEntities.Credential, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	return s.Credential, s.Err
}

// Requests returns every request Resolve received.
func (s *CredentialResolverStub) Requests() []credentialEntities.CredentialRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}
