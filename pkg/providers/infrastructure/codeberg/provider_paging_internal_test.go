package codeberg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// pagingRequestBudget is how many requests repeatingPage answers before it
// fails them: a walk with no guard against a server that ignores `page` then
// fails its test instead of hanging the suite.
const pagingRequestBudget = 5

// repeatingPage answers every request with items, whatever page it asks for,
// as a server that ignores `page` would, counting requests in served.
func repeatingPage(served *atomic.Int32, items []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if served.Add(1) > pagingRequestBudget {
			http.Error(w, "request budget exhausted", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	}
}

// repoItems, tagItems and pullItems return a full page of repositories, tags
// and open pull requests, none of the pull requests from the branch the tests
// look for.
func repoItems() []map[string]any {
	items := make([]map[string]any, 0, perPage)
	for id := 1; id <= perPage; id++ {
		items = append(items, map[string]any{"id": id, "name": fmt.Sprintf("repo-%02d", id), "default_branch": "main"})
	}
	return items
}

func tagItems() []map[string]any {
	items := make([]map[string]any, 0, perPage)
	for i := 1; i <= perPage; i++ {
		items = append(items, map[string]any{"name": fmt.Sprintf("v1.0.%d", i)})
	}
	return items
}

func pullItems() []map[string]any {
	items := make([]map[string]any, 0, perPage)
	for number := 1; number <= perPage; number++ {
		items = append(items, map[string]any{
			"number": number,
			"head":   map[string]any{"ref": fmt.Sprintf("feature-%02d", number)},
		})
	}
	return items
}

func TestDiscoverRepositoriesStopsOnARepeatedPage(t *testing.T) {
	t.Parallel()

	t.Run("should stop at the first repeated page of an organization's repositories", func(t *testing.T) {
		t.Parallel()

		// given
		var served atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/orgs/my-org/repos", repeatingPage(&served, repoItems()))
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		assert.Len(t, repos, perPage)
		assert.Equal(t, int32(2), served.Load(), "the first page that adds nothing new must end the walk")
	})

	t.Run("should stop at the first repeated page of a user's repositories", func(t *testing.T) {
		t.Parallel()

		// given: no organization by that name, so discovery falls back to the user.
		var served atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/orgs/my-user/repos", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		})
		mux.HandleFunc("GET /api/v1/users/my-user/repos", repeatingPage(&served, repoItems()))
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-user")

		// then
		require.NoError(t, err)
		assert.Len(t, repos, perPage)
		assert.Equal(t, int32(2), served.Load(), "the first page that adds nothing new must end the walk")
	})
}

func TestGetTagsStopsOnARepeatedPage(t *testing.T) {
	t.Parallel()

	t.Run("should stop at the first repeated page of tags", func(t *testing.T) {
		t.Parallel()

		// given
		var served atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/tags", repeatingPage(&served, tagItems()))
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		tags, err := p.GetTags(context.Background(), repo)

		// then
		require.NoError(t, err)
		assert.Len(t, tags, perPage)
		assert.Equal(t, int32(2), served.Load(), "the first page that adds nothing new must end the walk")
	})
}

func TestPullRequestExistsStopsOnARepeatedPage(t *testing.T) {
	t.Parallel()

	t.Run("should stop at the first repeated page of open pull requests", func(t *testing.T) {
		t.Parallel()

		// given
		var served atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/pulls", repeatingPage(&served, pullItems()))
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		exists, err := p.PullRequestExists(context.Background(), repo, "chore/bump-1.2.3")

		// then
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Equal(t, int32(2), served.Load(), "the first page that adds nothing new must end the walk")
	})
}
