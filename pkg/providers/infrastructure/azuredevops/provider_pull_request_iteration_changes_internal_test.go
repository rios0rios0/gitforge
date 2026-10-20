package azuredevops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

const (
	// changesPullRequestID and changesIterationID identify the pull request, and
	// its latest iteration, whose changes these tests list.
	changesPullRequestID = 42
	changesIterationID   = 3

	// changesPullRequestPath is the path of that pull request, and the two routes
	// below are its iteration listing and the changes listing of iteration 3.
	changesPullRequestPath     = "/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/42"
	pullRequestIterationsRoute = "GET " + changesPullRequestPath + "/iterations"
	iterationChangesRoute      = "GET " + changesPullRequestPath + "/iterations/3/changes"

	// fakeChangesPageCap is the most changes the fake listing serves for one
	// request, whatever `$top` asks for: the 100 Azure DevOps serves when `$top` is
	// omitted. A few hundred files then span several pages without a 2000-entry
	// fixture, and the walk has to follow the nextSkip/nextTop cursor instead of
	// assuming the page size it asked for.
	fakeChangesPageCap = 100
)

// iterationChangesListing fakes the changes listing of one pull request
// iteration. Its change at index i is an edit of changedFilePath(i) with
// changeTrackingId i+1, and it records the query of every request it answers.
type iterationChangesListing struct {
	mu      sync.Mutex
	count   int
	queries []url.Values
}

// newIterationChangesListing returns a listing of count changed files.
func newIterationChangesListing(count int) *iterationChangesListing {
	return &iterationChangesListing{count: count}
}

// answer records a request and returns its 1-based sequence number.
func (l *iterationChangesListing) answer(query url.Values) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.queries = append(l.queries, query)
	return len(l.queries)
}

// requests returns how many requests the listing has answered.
func (l *iterationChangesListing) requests() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.queries)
}

// queryValues returns the value of key in every request answered so far, in order.
func (l *iterationChangesListing) queryValues(key string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	values := make([]string, 0, len(l.queries))
	for _, query := range l.queries {
		values = append(values, query.Get(key))
	}
	return values
}

// pageFor returns the page a request selects with `$top` and `$skip`, capped at
// fakeChangesPageCap, with the cursor Azure DevOps documents: nextSkip and
// nextTop point at the next page and are both zero after the last one. A missing
// or malformed `$top` reads as the 100 the endpoint defaults to.
func (l *iterationChangesListing) pageFor(query url.Values) iterationChangesPage {
	top, err := strconv.Atoi(query.Get("$top"))
	if err != nil {
		top = fakeChangesPageCap
	}
	skip, _ := strconv.Atoi(query.Get("$skip"))

	first := min(max(skip, 0), l.count)
	page := iterationChangesPage{first: first, last: min(first+min(max(top, 0), fakeChangesPageCap), l.count)}
	if page.last < l.count {
		page.nextSkip, page.nextTop = page.last, fakeChangesPageCap
	}
	return page
}

// iterationChangesPage is one page of the fake listing: the changes at indexes
// first up to, but not including, last, and the cursor to the next page.
type iterationChangesPage struct {
	first, last       int
	nextSkip, nextTop int
}

// writeIterationChangesPage answers with page in the shape Azure DevOps serves it.
func writeIterationChangesPage(w http.ResponseWriter, page iterationChangesPage) {
	entries := make([]map[string]any, 0, page.last-page.first)
	for i := page.first; i < page.last; i++ {
		entries = append(entries, map[string]any{
			"changeTrackingId": i + 1,
			"changeId":         i + 1,
			"changeType":       "edit",
			"item":             map[string]any{"path": changedFilePath(i)},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"changeEntries": entries,
		"nextSkip":      page.nextSkip,
		"nextTop":       page.nextTop,
	})
}

// changedFilePath returns the path of the change at index i of the fake listing.
func changedFilePath(i int) string {
	return fmt.Sprintf("/src/file-%03d.go", i)
}

// changedFiles returns what GetPullRequestFiles maps the first count changes of
// the fake listing to.
func changedFiles(count int) []globalEntities.PullRequestFile {
	files := make([]globalEntities.PullRequestFile, 0, count)
	for i := range count {
		files = append(files, globalEntities.PullRequestFile{Path: changedFilePath(i), Status: "modified"})
	}
	return files
}

// servePullRequestIterations answers the iteration listing of the test pull
// request, whose latest iteration is changesIterationID.
func servePullRequestIterations(mux *http.ServeMux) {
	mux.HandleFunc(pullRequestIterationsRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{{"id": 1}, {"id": 2}, {"id": changesIterationID}},
		})
	})
}

func TestGetPullRequestFilesPagesThroughEveryChange(t *testing.T) {
	t.Parallel()

	t.Run("should return every changed file when they span more than one page", func(t *testing.T) {
		t.Parallel()

		// given
		count := 2*fakeChangesPageCap + fakeChangesPageCap/2
		listing := newIterationChangesListing(count)
		mux := http.NewServeMux()
		servePullRequestIterations(mux)
		mux.HandleFunc(iterationChangesRoute, func(w http.ResponseWriter, r *http.Request) {
			listing.answer(r.URL.Query())
			writeIterationChangesPage(w, listing.pageFor(r.URL.Query()))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		files, err := p.GetPullRequestFiles(context.Background(), repo, changesPullRequestID)

		// then
		require.NoError(t, err)
		assert.Equal(t, changedFiles(count), files)
		onePage := strconv.Itoa(fakeChangesPageCap)
		assert.Equal(t, []string{"0", onePage, strconv.Itoa(2 * fakeChangesPageCap)}, listing.queryValues("$skip"),
			"every page after the first must be read at the nextSkip the page before it returned")
		assert.Equal(t, []string{strconv.Itoa(pullRequestChangesPageSize), onePage, onePage},
			listing.queryValues("$top"),
			"the first page must ask for the most the endpoint allows, and the rest for the nextTop of the cursor")
	})
}

func TestGetChangeTrackingIDReadsEveryPage(t *testing.T) {
	t.Parallel()

	t.Run("should return the changeTrackingId of a file past the first page", func(t *testing.T) {
		t.Parallel()

		// given
		count := 2*fakeChangesPageCap + fakeChangesPageCap/2
		listing := newIterationChangesListing(count)
		mux := http.NewServeMux()
		mux.HandleFunc(iterationChangesRoute, func(w http.ResponseWriter, r *http.Request) {
			listing.answer(r.URL.Query())
			writeIterationChangesPage(w, listing.pageFor(r.URL.Query()))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}
		lastFile := count - 1

		// when
		changeTrackingID, found, err := p.getChangeTrackingID(
			context.Background(), repo, changesPullRequestID, changesIterationID,
			strings.TrimPrefix(changedFilePath(lastFile), "/"),
		)

		// then
		require.NoError(t, err)
		assert.True(t, found, "a file on the last page must be found")
		assert.JSONEq(t, strconv.Itoa(lastFile+1), string(changeTrackingID))
	})
}

func TestGetPullRequestFilesDeduplicatesAcrossPages(t *testing.T) {
	t.Parallel()

	t.Run("should list a file once when the cursor makes two pages overlap", func(t *testing.T) {
		t.Parallel()

		// given: the cursor points one change back into the page just served, so
		// the next page starts with the file this one ended with.
		count := fakeChangesPageCap + fakeChangesPageCap/2
		listing := newIterationChangesListing(count)
		mux := http.NewServeMux()
		servePullRequestIterations(mux)
		mux.HandleFunc(iterationChangesRoute, func(w http.ResponseWriter, r *http.Request) {
			listing.answer(r.URL.Query())
			page := listing.pageFor(r.URL.Query())
			if page.nextTop > 0 {
				page.nextSkip--
			}
			writeIterationChangesPage(w, page)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		files, err := p.GetPullRequestFiles(context.Background(), repo, changesPullRequestID)

		// then
		require.NoError(t, err)
		assert.Equal(t, changedFiles(count), files,
			"the file served on both pages must be listed once, and none may go missing")
		assert.Equal(t, 2, listing.requests())
	})
}

func TestGetPullRequestFilesStopsOnARepeatedPage(t *testing.T) {
	t.Parallel()

	t.Run("should stop paging when the server ignores the paging parameters and repeats a page", func(t *testing.T) {
		t.Parallel()

		// given: every request is answered with the first page and a cursor to the
		// second, as by a server that ignores `$skip` and `$top`. A walk without a
		// guard would follow that cursor forever, so requests past a small budget
		// fail instead of hanging the suite.
		const requestBudget = 5
		listing := newIterationChangesListing(3 * fakeChangesPageCap)
		mux := http.NewServeMux()
		servePullRequestIterations(mux)
		mux.HandleFunc(iterationChangesRoute, func(w http.ResponseWriter, r *http.Request) {
			if listing.answer(r.URL.Query()) > requestBudget {
				http.Error(w, "request budget exhausted", http.StatusInternalServerError)
				return
			}
			writeIterationChangesPage(w, listing.pageFor(url.Values{}))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		files, err := p.GetPullRequestFiles(context.Background(), repo, changesPullRequestID)

		// then
		require.NoError(t, err)
		assert.Equal(t, changedFiles(fakeChangesPageCap), files)
		assert.Equal(t, 2, listing.requests(), "the first page that adds nothing new must end the walk")
	})
}

func TestGetPullRequestFilesReturnsLaterPageErrors(t *testing.T) {
	t.Parallel()

	t.Run("should return the error rather than a partial file list when a later page fails", func(t *testing.T) {
		t.Parallel()

		// given
		listing := newIterationChangesListing(2*fakeChangesPageCap + fakeChangesPageCap/2)
		mux := http.NewServeMux()
		servePullRequestIterations(mux)
		mux.HandleFunc(iterationChangesRoute, func(w http.ResponseWriter, r *http.Request) {
			if listing.answer(r.URL.Query()) == 2 {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			writeIterationChangesPage(w, listing.pageFor(r.URL.Query()))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		files, err := p.GetPullRequestFiles(context.Background(), repo, changesPullRequestID)

		// then
		require.ErrorContains(t, err, "failed to get pull request iteration changes")
		require.ErrorContains(t, err, "status 500")
		assert.Nil(t, files, "the pages served before the failure must not come back as if they were the whole list")
		assert.Equal(t, 2, listing.requests(), "nothing may be asked for after the page that failed")
	})
}
