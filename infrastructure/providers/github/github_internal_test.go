package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v66/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
)

// newTestProvider creates a Provider backed by the given test server.
func newTestProvider(t *testing.T, server *httptest.Server) *Provider {
	t.Helper()
	client := gh.NewClient(server.Client()).WithAuthToken("test-token")
	baseURL := server.URL + "/"
	client.BaseURL, _ = client.BaseURL.Parse(baseURL)
	return &Provider{
		token:  "test-token",
		client: client,
	}
}

func TestConfigureTransport(t *testing.T) {
	t.Parallel()

	t.Run("should not panic", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test"}

		// when / then
		assert.NotPanics(t, func() {
			p.ConfigureTransport()
		})
	})
}

func TestDiscoverRepositoriesInternal(t *testing.T) {
	t.Parallel()

	t.Run("should discover org repos", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /orgs/my-org/repos", func(w http.ResponseWriter, _ *http.Request) {
			repos := []map[string]interface{}{
				{
					"id":             1,
					"name":           "repo-a",
					"clone_url":      "https://github.com/my-org/repo-a.git",
					"ssh_url":        "git@github.com:my-org/repo-a.git",
					"default_branch": "main",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(repos)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		require.Len(t, repos, 1)
		assert.Equal(t, "repo-a", repos[0].Name)
		assert.Equal(t, providerName, repos[0].ProviderName)
	})

	t.Run("should fall back to user repos when org listing fails", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /orgs/my-user/repos", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		})
		mux.HandleFunc("GET /users/my-user/repos", func(w http.ResponseWriter, _ *http.Request) {
			repos := []map[string]interface{}{
				{
					"id":             2,
					"name":           "user-repo",
					"clone_url":      "https://github.com/my-user/user-repo.git",
					"ssh_url":        "git@github.com:my-user/user-repo.git",
					"default_branch": "master",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(repos)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-user")

		// then
		require.NoError(t, err)
		require.Len(t, repos, 1)
		assert.Equal(t, "user-repo", repos[0].Name)
	})
}

func TestCreatePullRequestInternal(t *testing.T) {
	t.Parallel()

	t.Run("should create pull request successfully", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("POST /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			pr := map[string]interface{}{
				"number":   42,
				"title":    "Test PR",
				"html_url": "https://github.com/my-org/my-repo/pull/42",
				"state":    "open",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(pr)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Name:         "my-repo",
		}
		input := entities.PullRequestInput{
			SourceBranch: "refs/heads/feature",
			TargetBranch: "refs/heads/main",
			Title:        "Test PR",
			Description:  "Test description",
		}

		// when
		pr, err := p.CreatePullRequest(context.Background(), repo, input)

		// then
		require.NoError(t, err)
		require.NotNil(t, pr)
		assert.Equal(t, 42, pr.ID)
		assert.Equal(t, "Test PR", pr.Title)
		assert.Equal(t, "open", pr.Status)
	})
}

func TestPullRequestExistsInternal(t *testing.T) {
	t.Parallel()

	t.Run("should return true when PRs exist", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			prs := []map[string]interface{}{
				{"number": 1, "title": "Existing PR"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(prs)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		exists, err := p.PullRequestExists(context.Background(), repo, "feature-branch")

		// then
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should return false when no PRs exist", func(t *testing.T) {
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
		repo := entities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		exists, err := p.PullRequestExists(context.Background(), repo, "feature-branch")

		// then
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestGetFileContentInternal(t *testing.T) {
	t.Parallel()

	t.Run("should get file content from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/contents/README.md", func(w http.ResponseWriter, _ *http.Request) {
			// GitHub returns base64-encoded content
			resp := map[string]interface{}{
				"type":     "file",
				"encoding": "base64",
				"content":  "SGVsbG8gV29ybGQ=", // "Hello World" base64
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization:  "my-org",
			Name:          "my-repo",
			DefaultBranch: "refs/heads/main",
		}

		// when
		content, err := p.GetFileContent(context.Background(), repo, "README.md")

		// then
		require.NoError(t, err)
		assert.Equal(t, "Hello World", content)
	})
}

func TestListFilesInternal(t *testing.T) {
	t.Parallel()

	t.Run("should list files from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/git/trees/main", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"tree": []map[string]interface{}{
					{"path": "README.md", "type": "blob", "sha": "abc123"},
					{"path": "src", "type": "tree", "sha": "def456"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization:  "my-org",
			Name:          "my-repo",
			DefaultBranch: "refs/heads/main",
		}

		// when
		files, err := p.ListFiles(context.Background(), repo, "")

		// then
		require.NoError(t, err)
		assert.Len(t, files, 2)
	})
}

func TestGetTagsInternal(t *testing.T) {
	t.Parallel()

	t.Run("should get tags from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/tags", func(w http.ResponseWriter, _ *http.Request) {
			resp := []map[string]interface{}{
				{"name": "v1.0.0", "commit": map[string]string{"sha": "abc"}},
				{"name": "v2.0.0", "commit": map[string]string{"sha": "def"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		tags, err := p.GetTags(context.Background(), repo)

		// then
		require.NoError(t, err)
		assert.Len(t, tags, 2)
	})
}

func TestHasFileInternal(t *testing.T) {
	t.Parallel()

	t.Run("should return true when file exists", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/contents/README.md", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"type":     "file",
				"encoding": "base64",
				"content":  "SGVsbG8=",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization:  "my-org",
			Name:          "my-repo",
			DefaultBranch: "refs/heads/main",
		}

		// when
		result := p.HasFile(context.Background(), repo, "README.md")

		// then
		assert.True(t, result)
	})

	t.Run("should return false when file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/contents/missing.md", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization:  "my-org",
			Name:          "my-repo",
			DefaultBranch: "refs/heads/main",
		}

		// when
		result := p.HasFile(context.Background(), repo, "missing.md")

		// then
		assert.False(t, result)
	})
}

func TestGithubRepoToDomain(t *testing.T) {
	t.Parallel()

	t.Run("should convert github repo to domain entity", func(t *testing.T) {
		t.Parallel()

		// given
		id := int64(123)
		name := "my-repo"
		cloneURL := "https://github.com/my-org/my-repo.git"
		sshURL := "git@github.com:my-org/my-repo.git"
		defaultBranch := "develop"
		ghRepo := &gh.Repository{
			ID:            &id,
			Name:          &name,
			CloneURL:      &cloneURL,
			SSHURL:        &sshURL,
			DefaultBranch: &defaultBranch,
		}

		// when
		result := githubRepoToDomain(ghRepo, "my-org")

		// then
		assert.Equal(t, "123", result.ID)
		assert.Equal(t, "my-repo", result.Name)
		assert.Equal(t, "my-org", result.Organization)
		assert.Equal(t, "refs/heads/develop", result.DefaultBranch)
		assert.Equal(t, cloneURL, result.RemoteURL)
		assert.Equal(t, sshURL, result.SSHURL)
		assert.Equal(t, providerName, result.ProviderName)
	})

	t.Run("should use main as default branch when nil", func(t *testing.T) {
		t.Parallel()

		// given
		id := int64(456)
		name := "other-repo"
		ghRepo := &gh.Repository{
			ID:   &id,
			Name: &name,
		}

		// when
		result := githubRepoToDomain(ghRepo, "org")

		// then
		assert.Equal(t, "refs/heads/main", result.DefaultBranch)
	})
}
