package azuredevops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// pullRequestListingRoute is the pull request listing route of the repository these tests use.
const pullRequestListingRoute = "GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests"

// pullRequestListing fakes the Azure DevOps pull request listing. It holds the
// active pull request IDs in the order the endpoint lists them and records the
// query of every request it answers, so a test can assert how the provider paged.
type pullRequestListing struct {
	mu      sync.Mutex
	ids     []int
	queries []url.Values
}

// newPullRequestListing returns a listing holding the IDs count down to 1.
func newPullRequestListing(count int) *pullRequestListing {
	return &pullRequestListing{ids: descendingPullRequestIDs(count)}
}

// answer records a request and returns its 1-based sequence number together
// with the IDs listed at that moment.
func (l *pullRequestListing) answer(query url.Values) (int, []int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.queries = append(l.queries, query)
	return len(l.queries), slices.Clone(l.ids)
}

// open lists a new pull request ahead of the others, as one opened between two
// pages of a walk.
func (l *pullRequestListing) open(id int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.ids = slices.Insert(l.ids, 0, id)
}

// requests returns how many requests the listing has answered.
func (l *pullRequestListing) requests() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.queries)
}

// queryValues returns the value of key in every request answered so far, in order.
func (l *pullRequestListing) queryValues(key string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	values := make([]string, 0, len(l.queries))
	for _, query := range l.queries {
		values = append(values, query.Get(key))
	}
	return values
}

// descendingPullRequestIDs returns the IDs count down to 1: newest first, the
// order the endpoint's documented examples list active pull requests in.
func descendingPullRequestIDs(count int) []int {
	ids := make([]int, 0, count)
	for id := count; id > 0; id-- {
		ids = append(ids, id)
	}
	return ids
}

// pageOf returns the IDs a request selects with `$top` and `$skip`. A missing or
// malformed value reads as 0, so a provider that stops sending one is served an
// empty page and the assertions on the full listing fail.
func pageOf(ids []int, query url.Values) []int {
	top, _ := strconv.Atoi(query.Get("$top"))
	skip, _ := strconv.Atoi(query.Get("$skip"))

	start := min(max(skip, 0), len(ids))
	end := min(start+max(top, 0), len(ids))
	return ids[start:end]
}

// writePullRequestsPage answers with one page of the listing holding ids.
func writePullRequestsPage(w http.ResponseWriter, ids []int) {
	value := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		value = append(value, map[string]any{
			"pullRequestId": id,
			"title":         "pull request " + strconv.Itoa(id),
			"status":        "active",
			"sourceRefName": "refs/heads/feature-" + strconv.Itoa(id),
			"targetRefName": "refs/heads/main",
			"createdBy":     map[string]any{"displayName": "alice"},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"value": value, "count": len(value)})
}

// pullRequestIDs returns the ID of every pull request in prs, in order.
func pullRequestIDs(prs []globalEntities.PullRequestDetail) []int {
	ids := make([]int, 0, len(prs))
	for _, pr := range prs {
		ids = append(ids, pr.ID)
	}
	return ids
}

func TestListOpenPullRequestsPagesThroughEveryPullRequest(t *testing.T) {
	t.Parallel()

	onePage := strconv.Itoa(pullRequestsPageSize)
	twoPages := strconv.Itoa(2 * pullRequestsPageSize)
	cases := []struct {
		name      string
		count     int
		wantSkips []string
	}{
		{
			name:      "should return every pull request when they fit in a single page",
			count:     pullRequestsPageSize / 2,
			wantSkips: []string{"0"},
		},
		{
			name:      "should return every pull request when they span more than one page",
			count:     2*pullRequestsPageSize + pullRequestsPageSize/2,
			wantSkips: []string{"0", onePage, twoPages},
		},
		{
			name:      "should stop at the empty page that follows when the last page is exactly full",
			count:     2 * pullRequestsPageSize,
			wantSkips: []string{"0", onePage, twoPages},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			listing := newPullRequestListing(tt.count)
			mux := http.NewServeMux()
			mux.HandleFunc(pullRequestListingRoute, func(w http.ResponseWriter, r *http.Request) {
				_, ids := listing.answer(r.URL.Query())
				writePullRequestsPage(w, pageOf(ids, r.URL.Query()))
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)
			repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

			// when
			prs, err := p.ListOpenPullRequests(context.Background(), repo)

			// then
			require.NoError(t, err)
			assert.Equal(t, descendingPullRequestIDs(tt.count), pullRequestIDs(prs))
			assert.Equal(t, tt.wantSkips, listing.queryValues("$skip"))
			assert.Equal(t, slices.Repeat([]string{onePage}, len(tt.wantSkips)), listing.queryValues("$top"),
				"every page must ask for an explicit $top; without one Azure DevOps serves a single page")
			assert.Equal(t, slices.Repeat([]string{"active"}, len(tt.wantSkips)),
				listing.queryValues("searchCriteria.status"))
		})
	}
}

func TestListOpenPullRequestsDeduplicatesAcrossPages(t *testing.T) {
	t.Parallel()

	t.Run("should list a pull request once when one opened mid-walk repeats it on the next page", func(t *testing.T) {
		t.Parallel()

		// given: a pull request is opened once the first page has been served,
		// which shifts every other one place along, so the second page starts
		// with the pull request the first page ended with.
		count := pullRequestsPageSize + pullRequestsPageSize/2
		listing := newPullRequestListing(count)
		mux := http.NewServeMux()
		mux.HandleFunc(pullRequestListingRoute, func(w http.ResponseWriter, r *http.Request) {
			request, ids := listing.answer(r.URL.Query())
			if request == 1 {
				listing.open(count + 1)
			}
			writePullRequestsPage(w, pageOf(ids, r.URL.Query()))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		prs, err := p.ListOpenPullRequests(context.Background(), repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, descendingPullRequestIDs(count), pullRequestIDs(prs),
			"the pull request served on both pages must be listed once, and none may go missing")
		assert.Equal(t, 2, listing.requests())
	})
}

func TestListOpenPullRequestsStopsOnARepeatedPage(t *testing.T) {
	t.Parallel()

	t.Run("should stop paging when the server ignores $skip and serves the same page again", func(t *testing.T) {
		t.Parallel()

		// given: every request is answered with the first page, as by a server that
		// does not implement $skip. A walk without a guard would ask again forever,
		// so requests past a small budget fail instead of hanging the suite.
		const requestBudget = 5
		count := 3 * pullRequestsPageSize
		listing := newPullRequestListing(count)
		mux := http.NewServeMux()
		mux.HandleFunc(pullRequestListingRoute, func(w http.ResponseWriter, r *http.Request) {
			request, ids := listing.answer(r.URL.Query())
			if request > requestBudget {
				http.Error(w, "request budget exhausted", http.StatusInternalServerError)
				return
			}
			writePullRequestsPage(w, ids[:pullRequestsPageSize])
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		prs, err := p.ListOpenPullRequests(context.Background(), repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, descendingPullRequestIDs(count)[:pullRequestsPageSize], pullRequestIDs(prs))
		assert.Equal(t, 2, listing.requests(), "the first page that adds nothing new must end the walk")
	})
}

func TestListOpenPullRequestsReturnsLaterPageErrors(t *testing.T) {
	t.Parallel()

	t.Run("should return the error rather than a partial list when a later page fails", func(t *testing.T) {
		t.Parallel()

		// given
		listing := newPullRequestListing(2*pullRequestsPageSize + pullRequestsPageSize/2)
		mux := http.NewServeMux()
		mux.HandleFunc(pullRequestListingRoute, func(w http.ResponseWriter, r *http.Request) {
			request, ids := listing.answer(r.URL.Query())
			if request == 2 {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			writePullRequestsPage(w, pageOf(ids, r.URL.Query()))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		prs, err := p.ListOpenPullRequests(context.Background(), repo)

		// then
		require.ErrorContains(t, err, "failed to list open pull requests")
		require.ErrorContains(t, err, "status 500")
		assert.Nil(t, prs, "the pages served before the failure must not come back as if they were the whole list")
		assert.Equal(t, 2, listing.requests(), "nothing may be asked for after the page that failed")
	})
}
