package azuredevops

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

	t.Run("should abandon the active pull request when one exists for the branch", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedMethod string
		var capturedBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"count": 1,
					"value": []map[string]any{{"pullRequestId": 99}},
				})
			},
		)
		mux.HandleFunc(
			"/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/99",
			func(w http.ResponseWriter, r *http.Request) {
				capturedMethod = r.Method
				defer func() { _ = r.Body.Close() }()
				_ = json.NewDecoder(r.Body).Decode(&capturedBody)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"pullRequestId":99,"status":"abandoned"}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		closed, err := p.ClosePullRequest(context.Background(), repo, "chore/bump-1.2.3")

		// then
		require.NoError(t, err)
		assert.True(t, closed)
		assert.Equal(t, http.MethodPatch, capturedMethod)
		assert.Equal(t, prStatusAbandoned, capturedBody[jsonKeyStatus])
	})

	t.Run("should report nothing closed when no active pull request exists", func(t *testing.T) {
		t.Parallel()

		// given
		abandonCalled := false
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"count": 0, "value": []map[string]any{}})
			},
		)
		mux.HandleFunc(
			"/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/",
			func(w http.ResponseWriter, _ *http.Request) {
				abandonCalled = true
				w.WriteHeader(http.StatusOK)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		closed, err := p.ClosePullRequest(context.Background(), repo, "chore/bump-1.2.3")

		// then
		require.NoError(t, err)
		assert.False(t, closed)
		assert.False(t, abandonCalled, "no pull request should be abandoned when none is active")
	})
}
