package gitlab

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

func TestClosePullRequestInternal(t *testing.T) {
	t.Parallel()

	t.Run("should return an error when the client is not initialised", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := globalEntities.Repository{Organization: "org", Name: "repo"}

		// when
		_, err := p.ClosePullRequest(context.Background(), repo, "chore/bump-1.2.3")

		// then
		require.Error(t, err)
	})

	t.Run("should close the opened merge request when one exists for the branch", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedMethod string
		var capturedBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]map[string]any{{"iid": 7, "title": "Stale bump"}})
				return
			}
			capturedMethod = r.Method
			defer func() { _ = r.Body.Close() }()
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			_, _ = w.Write([]byte(`{"iid":7,"state":"closed"}`))
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
		assert.Equal(t, http.MethodPut, capturedMethod)
		assert.Equal(t, "close", capturedBody["state_event"])
	})

	t.Run("should report nothing closed when no merge request is opened", func(t *testing.T) {
		t.Parallel()

		// given
		updateCalled := false
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte("[]"))
				return
			}
			updateCalled = true
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
		assert.False(t, updateCalled, "no merge request should be updated when none is opened")
	})
}
