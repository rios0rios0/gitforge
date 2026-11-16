package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

func TestFindOpenPullRequestInternal(t *testing.T) {
	t.Parallel()

	repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

	t.Run("should return the open pull request of the branch", func(t *testing.T) {
		t.Parallel()

		// given
		var head, state string
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, r *http.Request) {
			head = r.URL.Query().Get("head")
			state = r.URL.Query().Get("state")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"number": 7, "title": "chore(refresh): refreshed documentation",
				"html_url": "https://github.com/my-org/my-repo/pull/7", "state": "open",
			}})
		})
		server := httptest.NewServer(mux)
		defer server.Close()
		p := newTestProvider(t, server)

		// when
		pr, found, err := p.FindOpenPullRequest(context.Background(), repo, "chore/config-and-docs-refresh")

		// then
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, globalEntities.PullRequest{
			ID: 7, Title: "chore(refresh): refreshed documentation",
			URL: "https://github.com/my-org/my-repo/pull/7", Status: "open",
		}, pr)
		assert.Equal(t, "my-org:chore/config-and-docs-refresh", head)
		assert.Equal(t, "open", state)
	})

	t.Run("should report none when no pull request is open", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		})
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
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		})
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

func TestGithubRepoToDomainDisabledInternal(t *testing.T) {
	t.Parallel()

	t.Run("should report a disabled repository", func(t *testing.T) {
		t.Parallel()

		// given
		name := "frozen"
		disabled := true
		repository := &gh.Repository{Name: &name, Disabled: &disabled}

		// when
		result := githubRepoToDomain(repository, "my-org")

		// then
		assert.True(t, result.IsDisabled)
		assert.False(t, result.IsArchived)
	})
}
