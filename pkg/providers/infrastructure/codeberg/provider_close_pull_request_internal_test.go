package codeberg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// newTestProvider creates a Provider pointed at the given test server.
func newTestProvider(t *testing.T, server *httptest.Server) *Provider {
	t.Helper()
	return &Provider{
		token:      "test-token",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}
}

func TestClosePullRequestInternal(t *testing.T) {
	t.Parallel()

	t.Run("should close the open pull request when one exists for the branch", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedMethod string
		var capturedBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"number":5,"title":"Stale bump","head":{"ref":"chore/bump-1.2.3"}}]`))
		})
		mux.HandleFunc("/api/v1/repos/my-org/my-repo/pulls/5", func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			defer func() { _ = r.Body.Close() }()
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":5,"state":"closed"}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		closed, err := p.ClosePullRequest(context.Background(), repo, "chore/bump-1.2.3")

		// then
		require.NoError(t, err)
		assert.True(t, closed)
		assert.Equal(t, http.MethodPatch, capturedMethod)
		assert.Equal(t, "closed", capturedBody["state"])
	})

	t.Run("should report nothing closed when no pull request matches the branch", func(t *testing.T) {
		t.Parallel()

		// given
		closeCalled := false
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"number":9,"title":"Other work","head":{"ref":"feat/unrelated"}}]`))
		})
		mux.HandleFunc("/api/v1/repos/my-org/my-repo/pulls/", func(w http.ResponseWriter, _ *http.Request) {
			closeCalled = true
			w.WriteHeader(http.StatusOK)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		closed, err := p.ClosePullRequest(context.Background(), repo, "chore/bump-1.2.3")

		// then
		require.NoError(t, err)
		assert.False(t, closed)
		assert.False(t, closeCalled, "no pull request should be closed when none matches the branch")
	})
}
