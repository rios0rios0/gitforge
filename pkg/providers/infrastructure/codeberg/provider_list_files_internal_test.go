package codeberg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

const (
	// treeCommitSHA is the commit the default branch "main" points at in these
	// tests; Forgejo reports it as the `sha` of every page of the tree listing.
	treeCommitSHA = "c0ffee0000000000000000000000000000000000"

	// forgejoTreesRoute is the Git trees route of the repository these tests list.
	forgejoTreesRoute = "GET /api/v1/repos/my-org/my-repo/git/trees/{ref}"

	// fakeTreesPerPageMax is the most entries the fake serves on one page,
	// whatever `per_page` asks for: an instance whose DEFAULT_GIT_TREES_PER_PAGE
	// is 100, so a few hundred entries span several pages.
	fakeTreesPerPageMax = 100
)

// forgejoTreeListing fakes Forgejo's Git trees listing of a repository whose
// default branch holds count files. It records the ref and the query of every
// request it answers.
type forgejoTreeListing struct {
	mu      sync.Mutex
	count   int
	refs    []string
	queries []url.Values
}

// newForgejoTreeListing returns a listing of count files.
func newForgejoTreeListing(count int) *forgejoTreeListing {
	return &forgejoTreeListing{count: count}
}

// answer records a request and returns its 1-based sequence number.
func (l *forgejoTreeListing) answer(r *http.Request) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refs = append(l.refs, r.PathValue("ref"))
	l.queries = append(l.queries, r.URL.Query())
	return len(l.queries)
}

// requestedRefs returns the ref every request answered so far asked for, in order.
func (l *forgejoTreeListing) requestedRefs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.refs...)
}

// queryValues returns the value of key in every request answered so far, in order.
func (l *forgejoTreeListing) queryValues(key string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	values := make([]string, 0, len(l.queries))
	for _, query := range l.queries {
		values = append(values, query.Get(key))
	}
	return values
}

// treePage builds the answer to one page request the way Forgejo's
// GetTreeBySHA does: `per_page` clamped to fakeTreesPerPageMax, `truncated` set
// on every page of a listing longer than one page, the last included,
// `total_count` the size of the whole listing, and a page past the end empty
// with `truncated` unset. documented answers by the contract the API reference
// states instead: `truncated` only while more entries follow, and no
// `total_count`.
func (l *forgejoTreeListing) treePage(query url.Values, documented bool) map[string]any {
	size, _ := strconv.Atoi(query.Get("per_page"))
	if size <= 0 || size > fakeTreesPerPageMax {
		size = fakeTreesPerPageMax
	}
	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	body := map[string]any{"sha": treeCommitSHA, "page": page, "truncated": false}
	if !documented {
		body["total_count"] = l.count
	}
	first := size * (page - 1)
	if first >= l.count {
		return body
	}

	last := min(first+size, l.count)
	entries := make([]map[string]any, 0, last-first)
	for i := first; i < last; i++ {
		entries = append(entries, map[string]any{"path": treeFilePath(i), "type": "blob", "sha": treeFileSHA(i)})
	}
	body["tree"] = entries
	body["truncated"] = l.count > size
	if documented {
		body["truncated"] = last < l.count
	}
	return body
}

// writeTreePage answers with body.
func writeTreePage(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// treeFilePath and treeFileSHA return the path and the blob SHA of the file at
// index i of the fake listing.
func treeFilePath(i int) string {
	return fmt.Sprintf("src/file-%03d.go", i)
}

func treeFileSHA(i int) string {
	return fmt.Sprintf("blob-%03d", i)
}

// treeFiles returns what ListFiles maps the first count files of the fake
// listing to.
func treeFiles(count int) []globalEntities.File {
	files := make([]globalEntities.File, 0, count)
	for i := range count {
		files = append(files, globalEntities.File{Path: treeFilePath(i), ObjectID: treeFileSHA(i)})
	}
	return files
}

// listFilesTestRepo is the repository these tests list.
func listFilesTestRepo() globalEntities.Repository {
	return globalEntities.Repository{Organization: "my-org", Name: "my-repo", DefaultBranch: "refs/heads/main"}
}

func TestListFilesReadsEveryTreePage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		documented bool
	}{
		{
			name:       "should list every file when Forgejo sets truncated on every page and reports total_count",
			documented: false,
		},
		{
			name:       "should list every file when truncated is unset on the last page and total_count is absent",
			documented: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			count := 2*fakeTreesPerPageMax + fakeTreesPerPageMax/2
			listing := newForgejoTreeListing(count)
			mux := http.NewServeMux()
			mux.HandleFunc(forgejoTreesRoute, func(w http.ResponseWriter, r *http.Request) {
				listing.answer(r)
				writeTreePage(w, listing.treePage(r.URL.Query(), tt.documented))
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

			// then
			require.NoError(t, err)
			assert.Equal(t, treeFiles(count), files)
			assert.Equal(t, []string{"main", treeCommitSHA, treeCommitSHA}, listing.requestedRefs(),
				"every page after the first must be read from the commit the first page reported")
			assert.Equal(t, []string{"1", "2", "3"}, listing.queryValues("page"))
			assert.Equal(t, []string{"1000", "1000", "1000"}, listing.queryValues("per_page"))
		})
	}
}

func TestListFilesFailsWhenTheTreeCannotBeReadCompletely(t *testing.T) {
	t.Parallel()

	t.Run("should fail rather than return a partial list when the server ignores the page", func(t *testing.T) {
		t.Parallel()

		// given: every request is answered with the first page, as by a server
		// that ignores `page`. A walk without a guard would ask again forever, so
		// requests past a small budget fail instead of hanging the suite.
		const requestBudget = 5
		listing := newForgejoTreeListing(2*fakeTreesPerPageMax + fakeTreesPerPageMax/2)
		mux := http.NewServeMux()
		mux.HandleFunc(forgejoTreesRoute, func(w http.ResponseWriter, r *http.Request) {
			if listing.answer(r) > requestBudget {
				http.Error(w, "request budget exhausted", http.StatusInternalServerError)
				return
			}
			writeTreePage(w, listing.treePage(url.Values{}, false))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

		// then
		require.ErrorIs(t, err, errIncompleteListing)
		assert.Nil(t, files)
		assert.Len(t, listing.requestedRefs(), 2, "the first repeated page must end the walk")
	})

	t.Run("should return the error of a later page rather than a partial list", func(t *testing.T) {
		t.Parallel()

		// given
		listing := newForgejoTreeListing(2*fakeTreesPerPageMax + fakeTreesPerPageMax/2)
		mux := http.NewServeMux()
		mux.HandleFunc(forgejoTreesRoute, func(w http.ResponseWriter, r *http.Request) {
			if listing.answer(r) == 2 {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			writeTreePage(w, listing.treePage(r.URL.Query(), false))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

		// then
		require.ErrorContains(t, err, "failed to get repo tree")
		require.ErrorContains(t, err, "status 500")
		assert.Nil(t, files)
		assert.Len(t, listing.requestedRefs(), 2, "nothing may be asked for after the page that failed")
	})
}
