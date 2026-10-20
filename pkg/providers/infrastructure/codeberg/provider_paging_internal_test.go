package codeberg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
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

// pagedListing serves items the way Forgejo pages a listing: `page` (1-based)
// and `limit` select the slice, and a page past the end is empty. It records
// the `page` and `limit` of every request, and fails requests past
// pagingRequestBudget so a walk that never ends fails its test.
type pagedListing struct {
	mu     sync.Mutex
	items  []map[string]any
	pages  []string
	limits []string
}

func (l *pagedListing) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		l.mu.Lock()
		l.pages = append(l.pages, query.Get("page"))
		l.limits = append(l.limits, query.Get("limit"))
		requests := len(l.pages)
		l.mu.Unlock()
		if requests > pagingRequestBudget {
			http.Error(w, "request budget exhausted", http.StatusInternalServerError)
			return
		}

		page, _ := strconv.Atoi(query.Get("page"))
		limit, _ := strconv.Atoi(query.Get("limit"))
		first := min(max(page-1, 0)*max(limit, 0), len(l.items))
		last := min(first+max(limit, 0), len(l.items))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(l.items[first:last])
	}
}

// requested returns the `page` and the `limit` of every request answered so far.
func (l *pagedListing) requested() ([]string, []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.pages...), append([]string(nil), l.limits...)
}

// everyLimit returns the `limit` every request of a walk must send: perPage.
func everyLimit(requests int) []string {
	limits := make([]string, requests)
	for i := range limits {
		limits[i] = strconv.Itoa(perPage)
	}
	return limits
}

// repoItems, tagItems and pullItems return count repositories, tags and open
// pull requests, all distinct, none of the pull requests from the branch the
// tests look for.
func repoItems(count int) []map[string]any {
	items := make([]map[string]any, 0, count)
	for id := 1; id <= count; id++ {
		items = append(items, map[string]any{"id": id, "name": fmt.Sprintf("repo-%03d", id), "default_branch": "main"})
	}
	return items
}

func tagItems(count int) []map[string]any {
	items := make([]map[string]any, 0, count)
	for i := 1; i <= count; i++ {
		items = append(items, map[string]any{"name": fmt.Sprintf("v1.0.%d", i)})
	}
	return items
}

func pullItems(count int) []map[string]any {
	items := make([]map[string]any, 0, count)
	for number := 1; number <= count; number++ {
		items = append(items, map[string]any{
			"number": number,
			"head":   map[string]any{"ref": fmt.Sprintf("feature-%03d", number)},
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
		mux.HandleFunc("GET /api/v1/orgs/my-org/repos", repeatingPage(&served, repoItems(perPage)))
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
		mux.HandleFunc("GET /api/v1/users/my-user/repos", repeatingPage(&served, repoItems(perPage)))
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
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/tags", repeatingPage(&served, tagItems(perPage)))
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
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/pulls", repeatingPage(&served, pullItems(perPage)))
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

func TestDiscoverRepositoriesReadsEveryPage(t *testing.T) {
	t.Parallel()

	t.Run("should read an organization's repositories page by page until a short page", func(t *testing.T) {
		t.Parallel()

		// given: two full pages and a short third.
		count := 2*perPage + perPage/5
		listing := &pagedListing{items: repoItems(count)}
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/orgs/my-org/repos", listing.handler())
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		require.Len(t, repos, count)
		assert.Equal(
			t,
			fmt.Sprintf("repo-%03d", count),
			repos[count-1].Name,
			"the last page's repositories must be read",
		)
		pages, limits := listing.requested()
		assert.Equal(t, []string{"1", "2", "3"}, pages)
		assert.Equal(t, everyLimit(len(pages)), limits)
	})
}

func TestGetTagsReadsEveryPage(t *testing.T) {
	t.Parallel()

	t.Run("should read the tags page by page until a short page", func(t *testing.T) {
		t.Parallel()

		// given: two full pages and a short third.
		count := 2*perPage + perPage/5
		listing := &pagedListing{items: tagItems(count)}
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/tags", listing.handler())
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		tags, err := p.GetTags(context.Background(), repo)

		// then
		require.NoError(t, err)
		require.Len(t, tags, count)
		assert.Contains(
			t,
			tags,
			fmt.Sprintf("v1.0.%d", count),
			"the last page's tags must be read (GetTags sorts them)",
		)
		pages, limits := listing.requested()
		assert.Equal(t, []string{"1", "2", "3"}, pages)
		assert.Equal(t, everyLimit(len(pages)), limits)
	})
}

func TestPullRequestExistsFindsALaterPage(t *testing.T) {
	t.Parallel()

	t.Run("should find an open pull request on a later page and stop reading there", func(t *testing.T) {
		t.Parallel()

		// given: three pages of open pull requests, the one from the branch
		// looked for in the middle of the second.
		items := pullItems(2*perPage + perPage/5)
		items[perPage+perPage/2]["head"] = map[string]any{"ref": "chore/bump-1.2.3"}
		listing := &pagedListing{items: items}
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/repos/my-org/my-repo/pulls", listing.handler())
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		exists, err := p.PullRequestExists(context.Background(), repo, "refs/heads/chore/bump-1.2.3")

		// then
		require.NoError(t, err)
		assert.True(t, exists)
		pages, limits := listing.requested()
		assert.Equal(t, []string{"1", "2"}, pages, "the page after the match must not be requested")
		assert.Equal(t, everyLimit(len(pages)), limits)
	})
}
