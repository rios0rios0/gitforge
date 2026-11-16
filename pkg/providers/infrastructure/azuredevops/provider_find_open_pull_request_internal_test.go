package azuredevops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

func TestFindOpenPullRequestInternal(t *testing.T) {
	t.Parallel()

	repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

	t.Run("should return the active pull request of the branch", func(t *testing.T) {
		t.Parallel()

		// given
		var sourceRef, status string
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, r *http.Request) {
				sourceRef = r.URL.Query().Get("searchCriteria.sourceRefName")
				status = r.URL.Query().Get("searchCriteria.status")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"count": 1,
					"value": []map[string]any{{
						"pullRequestId": 42, "title": "chore(refresh): refreshed documentation",
						"url": "https://dev.azure.com/my-org/_apis/git/pullRequests/42", "status": "active",
					}},
				})
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		pr, found, err := p.FindOpenPullRequest(context.Background(), repo, "chore/config-and-docs-refresh")

		// then
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, globalEntities.PullRequest{
			ID: 42, Title: "chore(refresh): refreshed documentation",
			URL: "https://dev.azure.com/my-org/_apis/git/pullRequests/42", Status: "active",
		}, pr)
		assert.Equal(t, "refs/heads/chore/config-and-docs-refresh", sourceRef)
		assert.Equal(t, "active", status)
	})

	t.Run("should report none when no pull request is active", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"count":0,"value":[]}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		pr, found, err := p.FindOpenPullRequest(context.Background(), repo, "chore/config-and-docs-refresh")

		// then
		require.NoError(t, err)
		assert.False(t, found)
		assert.Equal(t, globalEntities.PullRequest{}, pr)
	})

	t.Run("should surface a failed lookup", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		_, found, err := p.FindOpenPullRequest(context.Background(), repo, "chore/config-and-docs-refresh")

		// then
		require.Error(t, err)
		assert.False(t, found)
	})
}

func TestCreatePullRequestProjectWithSpaceInternal(t *testing.T) {
	t.Parallel()

	t.Run("should send a project name holding a space percent-encoded", func(t *testing.T) {
		t.Parallel()

		// given
		var escapedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			escapedPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"pullRequestId":7,"title":"t","url":"u","status":"active"}`))
		}))
		defer server.Close()
		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "My Project", ID: "repo-1"}

		// when
		pr, err := p.CreatePullRequest(context.Background(), repo, globalEntities.PullRequestInput{
			SourceBranch: "feature", TargetBranch: "main", Title: "t",
		})

		// then
		require.NoError(t, err)
		assert.Equal(t, 7, pr.ID)
		assert.Equal(t, "/my-org/My%20Project/_apis/git/repositories/repo-1/pullrequests", escapedPath)
	})
}
