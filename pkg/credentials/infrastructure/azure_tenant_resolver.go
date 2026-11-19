package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// azureDevOpsBaseURL is where Azure DevOps organizations are served.
const azureDevOpsBaseURL = "https://dev.azure.com"

// azureResourceTenantHeader carries the organization's tenant on any connectionData
// response, the anonymous authentication challenge included.
const azureResourceTenantHeader = "X-Vss-Resourcetenant"

// azureNoTenant is the tenant reported for an organization backed by personal Microsoft
// accounts, which has no Microsoft Entra tenant.
const azureNoTenant = "00000000-0000-0000-0000-000000000000"

// ErrTenantDiscovery reports that Azure DevOps did not answer the tenant lookup with a
// usable tenant.
var ErrTenantDiscovery = errors.New("tenant discovery failed")

// HTTPAzureTenantResolver reads the tenant of an Azure DevOps organization from its
// public connectionData endpoint. The request carries no credential: Azure DevOps
// returns the tenant on the authentication challenge itself.
type HTTPAzureTenantResolver struct {
	// BaseURL replaces https://dev.azure.com, for a local test server.
	BaseURL string
	// Client sends the request. Nil means a client with DefaultCLITimeout.
	Client *http.Client
}

// NewHTTPAzureTenantResolver creates a resolver that asks https://dev.azure.com.
func NewHTTPAzureTenantResolver() *HTTPAzureTenantResolver {
	return &HTTPAzureTenantResolver{}
}

// GetTenant returns the organization's Microsoft Entra tenant ID, or "" when the
// organization has none.
func (r *HTTPAzureTenantResolver) GetTenant(ctx context.Context, organization string) (string, error) {
	if organization == "" {
		return "", fmt.Errorf("%w: no organization given", ErrTenantDiscovery)
	}

	baseURL := r.BaseURL
	if baseURL == "" {
		baseURL = azureDevOpsBaseURL
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(organization) + "/_apis/connectionData"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create the tenant discovery request: %w", err)
	}

	client := http.Client{Timeout: DefaultCLITimeout}
	if r.Client != nil {
		client = *r.Client
	}
	// The challenge and a sign-in redirect carry the header themselves; following a
	// redirect would only lead somewhere that does not.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request the organization's tenant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("%w: Azure DevOps answered HTTP %d", ErrTenantDiscovery, resp.StatusCode)
	}
	tenant := resp.Header.Get(azureResourceTenantHeader)
	if tenant == "" || tenant == azureNoTenant {
		return "", nil
	}
	if !azureTenantIDPattern.MatchString(tenant) {
		return "", fmt.Errorf("%w: Azure DevOps returned an invalid tenant ID", ErrTenantDiscovery)
	}
	return tenant, nil
}
