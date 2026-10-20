package github

import (
	"context"
	"encoding/json"
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
	// checkStatusHeadSHA is the head commit of the pull request whose checks
	// these tests read, and checkSuitesRoute the check suite listing of it.
	checkStatusHeadSHA = "0123456789abcdef0123456789abcdef01234567"
	checkSuitesRoute   = "GET /repos/my-org/my-repo/commits/" + checkStatusHeadSHA + "/check-suites"

	// githubDefaultPerPage and githubMaxPerPage are the page size GitHub serves
	// the check suite listing in without `per_page`, and the most it serves.
	githubDefaultPerPage = 30
	githubMaxPerPage     = 100
)

// queryRecorder records the query of every request a fake listing answers.
type queryRecorder struct {
	mu      sync.Mutex
	queries []url.Values
}

// record stores query and returns how many requests have been answered, this
// one included.
func (q *queryRecorder) record(query url.Values) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queries = append(q.queries, query)
	return len(q.queries)
}

// requests returns how many requests have been answered.
func (q *queryRecorder) requests() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.queries)
}

// queryValues returns the value of key in every request answered so far, in order.
func (q *queryRecorder) queryValues(key string) []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	values := make([]string, 0, len(q.queries))
	for _, query := range q.queries {
		values = append(values, query.Get(key))
	}
	return values
}

// serveCheckStatusBasics answers the pull request lookup with
// checkStatusHeadSHA as its head, and the combined status of that commit with
// no statuses at all, so the verdict rests on the check suites alone.
func serveCheckStatusBasics(mux *http.ServeMux) {
	mux.HandleFunc("GET /repos/my-org/my-repo/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"number": 7, "head": map[string]any{"sha": checkStatusHeadSHA}})
	})
	mux.HandleFunc(
		"GET /repos/my-org/my-repo/commits/"+checkStatusHeadSHA+"/status",
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "pending", "total_count": 0})
		},
	)
}

// pageOfCheckSuites returns the IDs, 1 to count, of the suites a request
// selects with `page` and `per_page` the way GitHub pages them, and the number
// of the page after it, 0 after the last.
func pageOfCheckSuites(count int, query url.Values) ([]int, int) {
	size, err := strconv.Atoi(query.Get("per_page"))
	if err != nil || size <= 0 {
		size = githubDefaultPerPage
	}
	size = min(size, githubMaxPerPage)
	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page <= 0 {
		page = 1
	}

	first := min((page-1)*size, count)
	last := min(first+size, count)
	ids := make([]int, 0, last-first)
	for id := first + 1; id <= last; id++ {
		ids = append(ids, id)
	}
	if last < count {
		return ids, page + 1
	}
	return ids, 0
}

// writeCheckSuitesPage answers with the completed suites ids, each concluded
// "success" except failing, the size of the whole listing as total, and a Link
// header to page next unless next is 0.
func writeCheckSuitesPage(w http.ResponseWriter, r *http.Request, ids []int, total, failing, next int) {
	suites := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		conclusion := "success"
		if id == failing {
			conclusion = "failure"
		}
		suites = append(suites, map[string]any{"id": id, "status": "completed", "conclusion": conclusion})
	}

	if next > 0 {
		query := url.Values{"page": {strconv.Itoa(next)}, "per_page": {r.URL.Query().Get("per_page")}}
		link := url.URL{Scheme: "http", Host: r.Host, Path: r.URL.Path, RawQuery: query.Encode()}
		w.Header().Set("Link", "<"+link.String()+`>; rel="next"`)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"total_count": total, "check_suites": suites})
}

func TestGetPullRequestCheckStatusReadsEveryCheckSuite(t *testing.T) {
	t.Parallel()

	t.Run("should fail the commit on a failing check suite past the first page", func(t *testing.T) {
		t.Parallel()

		// given
		const count = githubMaxPerPage + 1
		listing := &queryRecorder{}
		mux := http.NewServeMux()
		serveCheckStatusBasics(mux)
		mux.HandleFunc(checkSuitesRoute, func(w http.ResponseWriter, r *http.Request) {
			listing.record(r.URL.Query())
			ids, next := pageOfCheckSuites(count, r.URL.Query())
			writeCheckSuitesPage(w, r, ids, count, count, next)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		passed, err := p.GetPullRequestCheckStatus(context.Background(), repo, 7)

		// then
		require.NoError(t, err)
		assert.False(t, passed, "the failing suite on the second page must fail the commit")
		assert.Equal(t, []string{"", "2"}, listing.queryValues("page"))
		assert.Equal(t, []string{"100", "100"}, listing.queryValues("per_page"),
			"every page must ask for the most suites GitHub serves at once")
	})
}

func TestGetPullRequestCheckStatusFailsOnARepeatedPage(t *testing.T) {
	t.Parallel()

	t.Run("should fail rather than judge the commit when the server repeats a page", func(t *testing.T) {
		t.Parallel()

		// given: every request is answered with the first page and a link to the
		// second, as by a server that ignores `page`. A walk without a guard would
		// follow that link forever, so requests past a small budget fail instead of
		// hanging the suite.
		const requestBudget = 5
		listing := &queryRecorder{}
		mux := http.NewServeMux()
		serveCheckStatusBasics(mux)
		mux.HandleFunc(checkSuitesRoute, func(w http.ResponseWriter, r *http.Request) {
			if listing.record(r.URL.Query()) > requestBudget {
				http.Error(w, "request budget exhausted", http.StatusInternalServerError)
				return
			}
			ids, _ := pageOfCheckSuites(2*githubMaxPerPage, url.Values{"per_page": {"100"}})
			writeCheckSuitesPage(w, r, ids, 2*githubMaxPerPage, 0, 2)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		passed, err := p.GetPullRequestCheckStatus(context.Background(), repo, 7)

		// then
		require.ErrorIs(t, err, errIncompleteListing)
		assert.False(t, passed)
		assert.Equal(t, 2, listing.requests(), "the first repeated page must end the walk")
	})
}

func TestGetPullRequestCheckStatusReturnsLaterPageErrors(t *testing.T) {
	t.Parallel()

	t.Run("should return the error of a later page rather than judge the commit on the first", func(t *testing.T) {
		t.Parallel()

		// given
		const count = githubMaxPerPage + 1
		listing := &queryRecorder{}
		mux := http.NewServeMux()
		serveCheckStatusBasics(mux)
		mux.HandleFunc(checkSuitesRoute, func(w http.ResponseWriter, r *http.Request) {
			if listing.record(r.URL.Query()) == 2 {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			ids, next := pageOfCheckSuites(count, r.URL.Query())
			writeCheckSuitesPage(w, r, ids, count, 0, next)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		passed, err := p.GetPullRequestCheckStatus(context.Background(), repo, 7)

		// then
		require.ErrorContains(t, err, "failed to list check suites")
		require.ErrorContains(t, err, "500")
		assert.False(t, passed)
		assert.Equal(t, 2, listing.requests(), "nothing may be asked for after the page that failed")
	})
}
