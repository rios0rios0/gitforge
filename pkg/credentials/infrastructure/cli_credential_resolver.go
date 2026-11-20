package infrastructure

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// DefaultCLITimeout bounds how long a forge's CLI, together with the Azure tenant lookup
// before it, may take to hand over a token. Issuing one can involve a network round trip
// (az contacts Microsoft Entra), and an unreachable endpoint must not hang the caller.
const DefaultCLITimeout = 30 * time.Second

// azureDevOpsResource is the fixed Microsoft Entra application ID of Azure DevOps. A token
// the Azure CLI issues for it is accepted wherever a personal access token is, which is
// what lets an `az login` session stand in for AZURE_DEVOPS_EXT_PAT.
const azureDevOpsResource = "499b84ac-1321-427f-aa17-267ca6975798"

// githubHost is the only host gitforge's GitHub provider talks to (api.github.com).
const githubHost = "github.com"

// azureTenantIDPattern matches a Microsoft Entra tenant ID, a GUID.
var azureTenantIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

// ProviderCLI describes the command-line tool that already holds a user's credentials for
// a forge: the executable to look for on PATH, the arguments that print a token on
// standard output, and the command that signs the user in when it cannot.
type ProviderCLI struct {
	Binary    string
	TokenArgs []string
	LoginHint string
}

// providerCLIs maps each forge to the CLI that can issue a token for it. A forge with an
// empty entry has none.
//
// GitHub names its host: gitforge's GitHub provider only talks to api.github.com, and a
// bare `gh auth token` answers for GH_HOST or the default host, which may be a GitHub
// Enterprise server whose token must not travel to github.com.
//
//nolint:gochecknoglobals // read-only lookup table
var providerCLIs = map[globalEntities.ServiceType]ProviderCLI{
	globalEntities.UNKNOWN: {},
	globalEntities.GITHUB: {
		Binary:    "gh",
		TokenArgs: []string{"auth", "token", "--hostname", githubHost},
		LoginHint: "gh auth login --hostname " + githubHost,
	},
	globalEntities.AZUREDEVOPS: {
		Binary: "az",
		TokenArgs: []string{
			"account", "get-access-token",
			"--resource", azureDevOpsResource,
			"--query", "accessToken",
			"--output", "tsv",
		},
		LoginHint: "az login",
	},
	// glab has no `auth token` command. `glab auth status --show-token` and `glab config get
	// token` print one, but gitforge reads a token from neither, so GitLab stays on
	// GITLAB_TOKEN and GL_TOKEN.
	globalEntities.GITLAB:     {},
	globalEntities.BITBUCKET:  {},
	globalEntities.CODECOMMIT: {},
	// Codeberg has no widely installed official CLI.
	globalEntities.CODEBERG: {},
}

// CLICredentialResolver asks a forge's own CLI for a token. Someone who has run
// `gh auth login` or `az login` is already authenticated for that forge, so requiring a
// separately exported token asks them to prove the same thing twice.
type CLICredentialResolver struct {
	// Runner finds and runs the CLI. Nil means an ExecCLIRunner.
	Runner entities.CLIRunner
	// TenantResolver finds an Azure DevOps organization's tenant. Nil means an
	// HTTPAzureTenantResolver.
	TenantResolver entities.AzureTenantResolver
	// Timeout bounds the tenant lookup and the CLI together. Zero means
	// DefaultCLITimeout.
	Timeout time.Duration
}

// NewCLICredentialResolver creates a resolver that runs the real gh and az found on PATH.
func NewCLICredentialResolver() *CLICredentialResolver {
	return &CLICredentialResolver{
		Runner:         NewExecCLIRunner(),
		TenantResolver: NewHTTPAzureTenantResolver(),
		Timeout:        DefaultCLITimeout,
	}
}

// ProviderCLIFor returns the CLI registered for a forge, and whether there is one. The
// returned TokenArgs are a copy.
func ProviderCLIFor(serviceType globalEntities.ServiceType) (ProviderCLI, bool) {
	cli, ok := providerCLIs[serviceType]
	if !ok || cli.Binary == "" {
		return ProviderCLI{}, false
	}
	cli.TokenArgs = slices.Clone(cli.TokenArgs)
	return cli, true
}

// Resolve asks the forge's CLI for a token: `gh auth token --hostname github.com` for
// GitHub, and `az account get-access-token` for the Azure DevOps resource, in the
// organization's Microsoft Entra tenant when the request names an organization.
func (r *CLICredentialResolver) Resolve(
	ctx context.Context, request entities.CredentialRequest,
) (entities.Credential, error) {
	cli, ok := ProviderCLIFor(request.ServiceType)
	if !ok {
		return entities.Credential{}, newUnavailableError(
			nil, "%s has no supported CLI to ask for a token", request.ServiceType.DisplayName(),
		)
	}

	runner := r.Runner
	if runner == nil {
		runner = NewExecCLIRunner()
	}
	path, err := runner.LookPath(cli.Binary)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return entities.Credential{}, newUnavailableError(nil, "the %s CLI is not installed", cli.Binary)
		}
		return entities.Credential{}, newUnavailableError(err, "the %s CLI cannot be run", cli.Binary)
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultCLITimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cli, err = r.scopeToOrganization(ctx, cli, request)
	if err != nil {
		return entities.Credential{}, err
	}

	// The command can fail for reasons other than being signed out -- a timeout, a
	// network error while issuing the token -- so the message reports what was observed
	// and leaves the cause to the wrapped error.
	token, err := runner.Output(ctx, path, cli.TokenArgs...)
	if err != nil {
		return entities.Credential{}, cliFailure(ctx, cli, err)
	}
	if token == "" {
		return entities.Credential{}, newUnavailableError(
			nil, "the %s CLI returned an empty token (authenticate with %q)", cli.Binary, cli.LoginHint,
		)
	}
	// A token never holds whitespace or control characters. Anything else is not a token
	// -- a warning printed on standard output, say -- and must not reach a request
	// header. It is not echoed either: it could be a token after all.
	if strings.IndexFunc(token, isNotTokenRune) >= 0 {
		return entities.Credential{}, newUnavailableError(
			nil, "the %s CLI printed something other than a single token", cli.Binary,
		)
	}

	return entities.Credential{Token: token, Source: cli.Binary + " CLI"}, nil
}

// scopeToOrganization adds the organization's Microsoft Entra tenant to an Azure CLI
// request. The signed-in account's default subscription can belong to another tenant,
// and a token from that tenant is refused by the organization.
func (r *CLICredentialResolver) scopeToOrganization(
	ctx context.Context, cli ProviderCLI, request entities.CredentialRequest,
) (ProviderCLI, error) {
	if request.ServiceType != globalEntities.AZUREDEVOPS || request.Organization == "" {
		return cli, nil
	}

	tenants := r.TenantResolver
	if tenants == nil {
		tenants = NewHTTPAzureTenantResolver()
	}
	tenant, err := tenants.GetTenant(ctx, request.Organization)
	if err != nil {
		return ProviderCLI{}, newUnavailableError(
			err, "could not find the Microsoft Entra tenant of Azure DevOps organization %q", request.Organization,
		)
	}
	if tenant == "" {
		return cli, nil
	}
	// The tenant becomes an argument of az, so only a GUID is accepted: anything else
	// could be read as an option.
	if !azureTenantIDPattern.MatchString(tenant) {
		return ProviderCLI{}, newUnavailableError(
			nil, "the tenant found for Azure DevOps organization %q is not a tenant ID", request.Organization,
		)
	}

	cli.TokenArgs = append(cli.TokenArgs, "--tenant", tenant)
	cli.LoginHint = "az login --tenant " + tenant + " --allow-no-subscriptions"
	return cli, nil
}

// cliFailure explains a CLI run that produced no token. A run cut short by the deadline
// or by the caller says so instead of suggesting a sign-in that would not have helped.
func cliFailure(ctx context.Context, cli ProviderCLI, err error) error {
	switch ctxErr := ctx.Err(); {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return newUnavailableError(err, "the %s CLI did not provide a token in time", cli.Binary)
	case ctxErr != nil:
		return newUnavailableError(err, "asking the %s CLI for a token was cancelled", cli.Binary)
	default:
		return newUnavailableError(
			err, "the %s CLI could not provide a token (authenticate with %q)", cli.Binary, cli.LoginHint,
		)
	}
}

// isNotTokenRune reports a character no token contains.
func isNotTokenRune(char rune) bool {
	return unicode.IsSpace(char) || unicode.IsControl(char)
}
