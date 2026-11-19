package infrastructure

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
)

// ChainCredentialResolver tries each resolver in order and returns the first credential
// one supplies. When every resolver declines, its error names the forge and lists every
// reason, so the user sees each way of authenticating rather than only the last one that
// failed.
type ChainCredentialResolver struct {
	Resolvers []entities.CredentialResolver
}

// NewChainCredentialResolver creates a chain over the given resolvers, in priority order.
func NewChainCredentialResolver(resolvers ...entities.CredentialResolver) *ChainCredentialResolver {
	return &ChainCredentialResolver{Resolvers: resolvers}
}

// NewDefaultCredentialResolver returns the standard chain: the forge's token environment
// variables first, so an explicitly exported token always overrides whichever account a
// CLI happens to be signed in to, then the forge's own CLI (gh for GitHub, az for Azure
// DevOps).
//
// Use it only after the caller's own configured token, if any, came up empty. The token
// it returns belongs to the operator running the program, so send it only to the forge it
// was issued for: gitforge's providers call fixed API hosts (api.github.com,
// dev.azure.com), which is safe, while a git remote's own host is not, because a remote
// that merely contains "github.com" in its URL would receive the token too.
func NewDefaultCredentialResolver() *ChainCredentialResolver {
	return NewChainCredentialResolver(NewEnvCredentialResolver(), NewCLICredentialResolver())
}

// Resolve returns the first credential with a token that a resolver supplies.
func (r *ChainCredentialResolver) Resolve(
	ctx context.Context, request entities.CredentialRequest,
) (entities.Credential, error) {
	forge := request.ServiceType.DisplayName()
	reasons := make([]error, 0, len(r.Resolvers))
	for _, resolver := range r.Resolvers {
		credential, err := resolver.Resolve(ctx, request)
		if err == nil && credential.Token != "" {
			log.Debugf("Resolved a %s credential from the %s", forge, credential.Source)
			return credential, nil
		}
		if err == nil {
			err = emptyTokenFailure(credential.Source)
		}
		reasons = append(reasons, err)
		if ctx.Err() != nil {
			break
		}
	}
	return entities.Credential{}, &noCredentialError{forge: forge, reasons: reasons}
}

// emptyTokenFailure explains a resolver that returned no token and no error, naming its
// source when it gave one.
func emptyTokenFailure(source string) error {
	if source == "" {
		return newUnavailableError(nil, "a credential source returned an empty token")
	}
	return newUnavailableError(nil, "the %s returned an empty token", source)
}
