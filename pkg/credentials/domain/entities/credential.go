// Package entities holds the contracts of the credentials bounded context: the token a
// forge accepts, the request naming the forge it is for, and the resolver consumers
// depend on to obtain one.
package entities

import (
	"context"
	"errors"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// ErrNoCredential reports that a resolver could not supply a credential. Every resolver
// in this bounded context returns an error that matches it with [errors.Is]; the error's
// text says why, naming each source that was tried, and never carries a token.
var ErrNoCredential = errors.New("no credential available")

// Credential is a token a forge accepts, together with a human-readable description of
// where it came from ("GITHUB_TOKEN environment variable", "gh CLI"). The source exists so
// a log line or an error can name what produced a token without ever printing the token.
//
// A token from a CLI can be short-lived: an Azure CLI token lasts about an hour, and az
// may return a cached one with minutes left. Resolve again for each run or organization
// rather than keeping one for a long batch.
type Credential struct {
	Token  string
	Source string
}

// CredentialRequest names the forge a credential is for.
type CredentialRequest struct {
	// ServiceType selects the forge.
	ServiceType globalEntities.ServiceType

	// Organization is the Azure DevOps organization the token will be used against. The
	// Azure CLI then issues the token in that organization's Microsoft Entra tenant,
	// which can differ from the tenant of the signed-in account's default subscription.
	// Empty keeps the Azure CLI's default tenant, and other forges ignore it.
	Organization string
}

// CredentialResolver obtains a credential for a forge. A resolver that cannot supply one
// returns an error matching ErrNoCredential that explains why, so a chain of resolvers
// can report every avenue the user has rather than only the last one it tried.
type CredentialResolver interface {
	Resolve(ctx context.Context, request CredentialRequest) (Credential, error)
}

// CLIRunner finds and runs the command-line tool of a forge. It is the seam that lets
// the CLI credential resolver be tested without gh or az installed.
type CLIRunner interface {
	// LookPath returns the absolute path of the named executable.
	LookPath(name string) (string, error)

	// Output runs the executable at path with args and returns its standard output with
	// surrounding whitespace trimmed. A failure's error must never carry standard output,
	// which is where a CLI prints the token.
	Output(ctx context.Context, path string, args ...string) (string, error)
}

// AzureTenantResolver finds the Microsoft Entra tenant that owns an Azure DevOps
// organization. It returns an empty tenant for an organization that has none, such as
// one backed by personal Microsoft accounts.
type AzureTenantResolver interface {
	GetTenant(ctx context.Context, organization string) (string, error)
}
