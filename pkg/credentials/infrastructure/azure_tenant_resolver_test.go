package infrastructure_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/infrastructure"
)

const tenantHeader = "X-Vss-Resourcetenant"

func tenantServer(t *testing.T, handler http.HandlerFunc) *infrastructure.HTTPAzureTenantResolver {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &infrastructure.HTTPAzureTenantResolver{BaseURL: server.URL, Client: server.Client()}
}

func TestHTTPAzureTenantResolverGetTenant(t *testing.T) {
	t.Parallel()

	t.Run("should return the tenant when Azure DevOps sends it on the authentication challenge", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := tenantServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(tenantHeader, fixtureTenant)
			w.WriteHeader(http.StatusUnauthorized)
		})

		// when
		tenant, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.NoError(t, err)
		assert.Equal(t, fixtureTenant, tenant)
	})

	t.Run("should ask the organization's connectionData endpoint without credentials", func(t *testing.T) {
		t.Parallel()

		// given
		var path, authorization string
		resolver := tenantServer(t, func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.EscapedPath()
			authorization = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		})

		// when
		_, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.NoError(t, err)
		assert.Equal(t, "/fixture-org/_apis/connectionData", path)
		assert.Empty(t, authorization)
	})

	t.Run("should escape the organization when it holds a path separator", func(t *testing.T) {
		t.Parallel()

		// given
		var path string
		resolver := tenantServer(t, func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.EscapedPath()
			w.WriteHeader(http.StatusOK)
		})

		// when
		_, err := resolver.GetTenant(t.Context(), "fixture/../org")

		// then
		require.NoError(t, err)
		assert.Equal(t, "/fixture%2F..%2Forg/_apis/connectionData", path)
	})

	t.Run("should read the tenant from a sign-in redirect instead of following it", func(t *testing.T) {
		t.Parallel()

		// given
		followed := false
		resolver := tenantServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sign-in" {
				followed = true
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set(tenantHeader, fixtureTenant)
			http.Redirect(w, r, "/sign-in", http.StatusFound)
		})

		// when
		tenant, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.NoError(t, err)
		assert.Equal(t, fixtureTenant, tenant)
		assert.False(t, followed)
	})

	t.Run("should return no tenant when the organization has none", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := tenantServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(tenantHeader, "00000000-0000-0000-0000-000000000000")
			w.WriteHeader(http.StatusUnauthorized)
		})

		// when
		tenant, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.NoError(t, err)
		assert.Empty(t, tenant)
	})

	t.Run("should return no tenant when Azure DevOps sends no tenant header", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := tenantServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// when
		tenant, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.NoError(t, err)
		assert.Empty(t, tenant)
	})

	t.Run("should fail when Azure DevOps answers with an error status", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := tenantServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		// when
		_, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.ErrorIs(t, err, infrastructure.ErrTenantDiscovery)
		assert.Contains(t, err.Error(), "HTTP 404")
	})

	t.Run("should fail when the tenant header is not a tenant ID", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := tenantServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(tenantHeader, "--not-a-tenant")
			w.WriteHeader(http.StatusUnauthorized)
		})

		// when
		_, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.ErrorIs(t, err, infrastructure.ErrTenantDiscovery)
	})

	t.Run("should fail when no organization is given", func(t *testing.T) {
		t.Parallel()

		// given
		resolver := infrastructure.NewHTTPAzureTenantResolver()

		// when
		_, err := resolver.GetTenant(t.Context(), "")

		// then
		require.ErrorIs(t, err, infrastructure.ErrTenantDiscovery)
	})

	t.Run("should fail when Azure DevOps cannot be reached", func(t *testing.T) {
		t.Parallel()

		// given
		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()
		resolver := &infrastructure.HTTPAzureTenantResolver{BaseURL: server.URL}

		// when
		_, err := resolver.GetTenant(t.Context(), "fixture-org")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request the organization's tenant")
	})
}
