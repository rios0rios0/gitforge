package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

// listFilesRootSHA is the root tree of the default branch in these tests, the
// one its truncated recursive listing reports.
const listFilesRootSHA = "root-tree"

// gitTreeLevel is one tree of the fake repository as GitHub lists it without
// recursion. fail answers it with an error instead.
type gitTreeLevel struct {
	entries   []map[string]any
	truncated bool
	fail      bool
}

// fakeGitTrees serves the Git trees of a repository whose default branch is
// "main": its recursive listing, cut short the way GitHub cuts a listing past
// its limit, and every tree in levels one level at a time by SHA. It records the
// tree each request asked for, with "?recursive" after a recursive one.
type fakeGitTrees struct {
	mu        sync.Mutex
	levels    map[string]gitTreeLevel
	requested []string
}

// newFakeGitTrees returns a fake serving levels.
func newFakeGitTrees(levels map[string]gitTreeLevel) *fakeGitTrees {
	return &fakeGitTrees{levels: levels}
}

// serve answers one Git tree request.
func (f *fakeGitTrees) serve(w http.ResponseWriter, r *http.Request) {
	sha := r.PathValue("sha")
	recursive := r.URL.Query().Has("recursive")
	level, found := f.lookup(sha, recursive)

	w.Header().Set("Content-Type", "application/json")
	if recursive {
		// Two of the repository's entries, then GitHub's limit.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sha":       listFilesRootSHA,
			"truncated": true,
			"tree":      []map[string]any{blobEntry("README.md", "readme"), treeEntry("src", "src-tree")},
		})
		return
	}
	if !found || level.fail {
		http.Error(w, "tree unavailable", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"sha": sha, "truncated": level.truncated, "tree": level.entries})
}

// lookup records a request and returns the level it asks for.
func (f *fakeGitTrees) lookup(sha string, recursive bool) (gitTreeLevel, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if recursive {
		f.requested = append(f.requested, sha+"?recursive")
		return gitTreeLevel{}, true
	}
	f.requested = append(f.requested, sha)
	level, found := f.levels[sha]
	return level, found
}

// requests returns the tree each request asked for, in order.
func (f *fakeGitTrees) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.requested...)
}

// blobEntry and treeEntry build a file and a directory entry of a tree listing.
func blobEntry(path, sha string) map[string]any {
	return map[string]any{"path": path, "mode": "100644", "type": "blob", "sha": sha}
}

func treeEntry(path, sha string) map[string]any {
	return map[string]any{"path": path, "mode": "040000", "type": "tree", "sha": sha}
}

// gitTreeFixture returns, one level at a time, a repository holding README.md,
// src/main.go, src/pkg/util.go and docs/index.md.
func gitTreeFixture() map[string]gitTreeLevel {
	return map[string]gitTreeLevel{
		listFilesRootSHA: {entries: []map[string]any{
			blobEntry("README.md", "readme"), treeEntry("src", "src-tree"), treeEntry("docs", "docs-tree"),
		}},
		"src-tree":  {entries: []map[string]any{blobEntry("main.go", "main"), treeEntry("pkg", "pkg-tree")}},
		"pkg-tree":  {entries: []map[string]any{blobEntry("util.go", "util")}},
		"docs-tree": {entries: []map[string]any{blobEntry("index.md", "index")}},
	}
}

// listFilesTestRepo is the repository these tests list.
func listFilesTestRepo() globalEntities.Repository {
	return globalEntities.Repository{Organization: "my-org", Name: "my-repo", DefaultBranch: "refs/heads/main"}
}

func TestListFilesWalksATruncatedTree(t *testing.T) {
	t.Parallel()

	t.Run("should list every entry when GitHub truncates the recursive listing", func(t *testing.T) {
		t.Parallel()

		// given
		trees := newFakeGitTrees(gitTreeFixture())
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/git/trees/{sha}", trees.serve)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

		// then
		require.NoError(t, err)
		assert.Equal(t, []globalEntities.File{
			{Path: "README.md", ObjectID: "readme"},
			{Path: "src", ObjectID: "src-tree", IsDir: true},
			{Path: "src/main.go", ObjectID: "main"},
			{Path: "src/pkg", ObjectID: "pkg-tree", IsDir: true},
			{Path: "src/pkg/util.go", ObjectID: "util"},
			{Path: "docs", ObjectID: "docs-tree", IsDir: true},
			{Path: "docs/index.md", ObjectID: "index"},
		}, files)
		assert.Equal(t,
			[]string{"main?recursive", listFilesRootSHA, "src-tree", "pkg-tree", "docs-tree"},
			trees.requests(),
			"the walk must start from the root tree the truncated listing reported, not from the branch")
	})
}

func TestListFilesFailsWhenATreeCannotBeListedCompletely(t *testing.T) {
	t.Parallel()

	t.Run("should fail rather than return a partial list when a level is truncated even without recursion",
		func(t *testing.T) {
			t.Parallel()

			// given
			levels := gitTreeFixture()
			src := levels["src-tree"]
			src.truncated = true
			levels["src-tree"] = src
			mux := http.NewServeMux()
			mux.HandleFunc("GET /repos/my-org/my-repo/git/trees/{sha}", newFakeGitTrees(levels).serve)
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

			// then
			require.ErrorIs(t, err, errIncompleteListing)
			assert.Nil(t, files)
		},
	)

	t.Run("should fail rather than walk forever when a tree contains one of its own ancestors", func(t *testing.T) {
		t.Parallel()

		// given
		levels := gitTreeFixture()
		pkg := levels["pkg-tree"]
		pkg.entries = append(pkg.entries, treeEntry("loop", "src-tree"))
		levels["pkg-tree"] = pkg
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/git/trees/{sha}", newFakeGitTrees(levels).serve)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

		// then
		require.ErrorIs(t, err, errIncompleteListing)
		assert.Nil(t, files)
	})

	t.Run("should return the error of a level that cannot be read rather than a partial list", func(t *testing.T) {
		t.Parallel()

		// given
		levels := gitTreeFixture()
		docs := levels["docs-tree"]
		docs.fail = true
		levels["docs-tree"] = docs
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/git/trees/{sha}", newFakeGitTrees(levels).serve)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		files, err := p.ListFiles(context.Background(), listFilesTestRepo(), "")

		// then
		require.ErrorContains(t, err, "failed to get repo tree docs-tree")
		assert.Nil(t, files)
	})
}
