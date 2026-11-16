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

// writeRepositories answers a repository listing with the given repositories.
func writeRepositories(w http.ResponseWriter, repositories []map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"value": repositories})
}

func TestListOrganizationRepositoriesInternal(t *testing.T) {
	t.Parallel()

	t.Run("should list every repository with its project and state in one call", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			writeRepositories(w, []map[string]any{
				{
					"id": "repo-1", "name": "Plain", "defaultBranch": "refs/heads/main",
					"project": map[string]string{"id": "proj-1", "name": "Project One"},
				},
				{
					"id": "repo-2", "name": "Forked", "defaultBranch": "refs/heads/main", "isFork": true,
					"project": map[string]string{"id": "proj-1", "name": "Project One"},
				},
				{
					"id": "repo-3", "name": "Disabled", "isDisabled": true,
					"project": map[string]string{"id": "proj-2", "name": "Project Two"},
				},
				{
					"id": "repo-4", "name": "Maintained", "defaultBranch": "refs/heads/main", "isInMaintenance": true,
					"project": map[string]string{"id": "proj-2", "name": "Project Two"},
				},
			})
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		repos, err := p.ListOrganizationRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		assert.Equal(t, []globalEntities.Repository{
			{
				ID: "repo-1", Name: "Plain", Organization: "my-org", Project: "Project One",
				DefaultBranch: "refs/heads/main", ProviderName: providerName,
			},
			{
				ID: "repo-2", Name: "Forked", Organization: "my-org", Project: "Project One",
				DefaultBranch: "refs/heads/main", ProviderName: providerName, IsFork: true,
			},
			{
				ID: "repo-3", Name: "Disabled", Organization: "my-org", Project: "Project Two",
				ProviderName: providerName, IsDisabled: true,
			},
			{
				ID: "repo-4", Name: "Maintained", Organization: "my-org", Project: "Project Two",
				DefaultBranch: "refs/heads/main", ProviderName: providerName, IsDisabled: true,
			},
		}, repos)
	})

	t.Run("should follow the continuation token until the listing ends", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/git/repositories", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("continuationToken") == "" {
				w.Header().Set(paginationHeader, "page 2")
				writeRepositories(w, []map[string]any{{"id": "repo-1", "name": "First"}})
				return
			}
			assert.Equal(t, "page 2", r.URL.Query().Get("continuationToken"))
			writeRepositories(w, []map[string]any{{"id": "repo-2", "name": "Second"}})
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		repos, err := p.ListOrganizationRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		require.Len(t, repos, 2)
		assert.Equal(t, "First", repos[0].Name)
		assert.Equal(t, "Second", repos[1].Name)
	})

	t.Run("should return an error instead of a partial listing when a page fails", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/git/repositories", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("continuationToken") == "" {
				w.Header().Set(paginationHeader, "page-2")
				writeRepositories(w, []map[string]any{{"id": "repo-1", "name": "First"}})
				return
			}
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		repos, err := p.ListOrganizationRepositories(context.Background(), "my-org")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 503")
		assert.Nil(t, repos)
	})

	t.Run("should surface the sign-in page as an authentication error", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(httpStatusNonAuthoritative)
			_, _ = w.Write([]byte("<html>Sign in</html>"))
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		repos, err := p.ListOrganizationRepositories(context.Background(), "my-org")

		// then
		require.ErrorIs(t, err, ErrAuthentication)
		assert.Nil(t, repos)
	})
}

func TestDiscoverRepositoriesStateInternal(t *testing.T) {
	t.Parallel()

	t.Run("should report forks and disabled repositories", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/projects", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]string{{"id": "proj-1", "name": "ProjectA"}},
			})
		})
		mux.HandleFunc("GET /my-org/proj-1/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			writeRepositories(w, []map[string]any{
				{"id": "repo-1", "name": "Forked", "isFork": true},
				{"id": "repo-2", "name": "Disabled", "isDisabled": true},
				{"id": "repo-3", "name": "Maintained", "isInMaintenance": true},
				{"id": "repo-4", "name": "Plain"},
			})
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		require.Len(t, repos, 4)
		assert.True(t, repos[0].IsFork)
		assert.False(t, repos[0].IsDisabled)
		assert.True(t, repos[1].IsDisabled)
		assert.True(t, repos[2].IsDisabled)
		assert.False(t, repos[3].IsFork)
		assert.False(t, repos[3].IsDisabled)
		assert.Equal(t, "ProjectA", repos[3].Project)
	})

	t.Run("should keep the projects it can list when one project fails", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/projects", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]string{{"id": "proj-1", "name": "Broken"}, {"id": "proj-2", "name": "Working"}},
			})
		})
		mux.HandleFunc("GET /my-org/proj-1/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		})
		mux.HandleFunc("GET /my-org/proj-2/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			writeRepositories(w, []map[string]any{{"id": "repo-1", "name": "Kept"}})
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		require.Len(t, repos, 1)
		assert.Equal(t, "Kept", repos[0].Name)
		assert.Equal(t, "Working", repos[0].Project)
	})
}
