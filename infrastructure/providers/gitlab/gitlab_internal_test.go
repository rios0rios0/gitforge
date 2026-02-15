package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gl "gitlab.com/gitlab-org/api/client-go"

	"github.com/rios0rios0/gitforge/domain/entities"
)

// newTestProvider creates a Provider backed by the given test server.
func newTestProvider(t *testing.T, server *httptest.Server) *Provider {
	t.Helper()
	client, err := gl.NewClient("test-token", gl.WithBaseURL(server.URL+"/api/v4"))
	require.NoError(t, err)
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

func TestGitlabProjectToDomain(t *testing.T) {
	t.Parallel()

	t.Run("should convert gitlab project to domain entity", func(t *testing.T) {
		t.Parallel()

		// given
		proj := &gl.Project{
			ID:             123,
			Path:           "my-project",
			DefaultBranch:  "develop",
			HTTPURLToRepo:  "https://gitlab.com/my-org/my-project.git",
			SSHURLToRepo:   "git@gitlab.com:my-org/my-project.git",
		}

		// when
		result := gitlabProjectToDomain(proj, "my-org")

		// then
		assert.Equal(t, "123", result.ID)
		assert.Equal(t, "my-project", result.Name)
		assert.Equal(t, "my-org", result.Organization)
		assert.Equal(t, "refs/heads/develop", result.DefaultBranch)
		assert.Equal(t, "https://gitlab.com/my-org/my-project.git", result.RemoteURL)
		assert.Equal(t, "git@gitlab.com:my-org/my-project.git", result.SSHURL)
		assert.Equal(t, providerName, result.ProviderName)
	})

	t.Run("should use main as default branch when empty", func(t *testing.T) {
		t.Parallel()

		// given
		proj := &gl.Project{
			ID:   456,
			Path: "other-project",
		}

		// when
		result := gitlabProjectToDomain(proj, "org")

		// then
		assert.Equal(t, "refs/heads/main", result.DefaultBranch)
	})
}

func TestDiscoverRepositoriesNilClient(t *testing.T) {
	t.Parallel()

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}

		// when
		_, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.Error(t, err)
		assert.Equal(t, errClientNotInitialized, err)
	})
}

func TestDiscoverRepositoriesInternal(t *testing.T) {
	t.Parallel()

	t.Run("should discover group repos", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v4/groups/my-org/projects", func(w http.ResponseWriter, _ *http.Request) {
			projects := []map[string]interface{}{
				{
					"id":               1,
					"path":             "repo-a",
					"http_url_to_repo": "https://gitlab.com/my-org/repo-a.git",
					"ssh_url_to_repo":  "git@gitlab.com:my-org/repo-a.git",
					"default_branch":   "main",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(projects)
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

	t.Run("should fall back to user projects when group listing fails", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v4/groups/my-user/projects", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "404 Group Not Found"}`))
		})
		mux.HandleFunc("GET /api/v4/projects", func(w http.ResponseWriter, _ *http.Request) {
			projects := []map[string]interface{}{
				{
					"id":               2,
					"path":             "user-repo",
					"http_url_to_repo": "https://gitlab.com/my-user/user-repo.git",
					"ssh_url_to_repo":  "git@gitlab.com:my-user/user-repo.git",
					"default_branch":   "master",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(projects)
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

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := entities.Repository{Organization: "org", Name: "repo"}
		input := entities.PullRequestInput{Title: "Test"}

		// when
		_, err := p.CreatePullRequest(context.Background(), repo, input)

		// then
		require.Error(t, err)
	})

	t.Run("should create merge request successfully", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				mr := map[string]interface{}{
					"iid":     42,
					"title":   "Test MR",
					"web_url": "https://gitlab.com/my-org/my-repo/-/merge_requests/42",
					"state":   "opened",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(mr)
			}
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{Organization: "my-org", Name: "my-repo"}
		input := entities.PullRequestInput{
			SourceBranch: "refs/heads/feature",
			TargetBranch: "refs/heads/main",
			Title:        "Test MR",
			Description:  "Test description",
		}

		// when
		pr, err := p.CreatePullRequest(context.Background(), repo, input)

		// then
		require.NoError(t, err)
		require.NotNil(t, pr)
		assert.Equal(t, 42, pr.ID)
		assert.Equal(t, "Test MR", pr.Title)
	})
}

func TestPullRequestExistsInternal(t *testing.T) {
	t.Parallel()

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := entities.Repository{Organization: "org", Name: "repo"}

		// when
		_, err := p.PullRequestExists(context.Background(), repo, "feature")

		// then
		require.Error(t, err)
	})

	t.Run("should return true when MRs exist", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				mrs := []map[string]interface{}{
					{"iid": 1, "title": "Existing MR"},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(mrs)
			}
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

	t.Run("should return false when no MRs exist", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			}
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

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := entities.Repository{Organization: "org", Name: "repo"}

		// when
		_, err := p.GetFileContent(context.Background(), repo, "README.md")

		// then
		require.Error(t, err)
	})

	t.Run("should get file content from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hello World"))
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

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := entities.Repository{Organization: "org", Name: "repo"}

		// when
		_, err := p.ListFiles(context.Background(), repo, "")

		// then
		require.Error(t, err)
	})

	t.Run("should list files from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, _ *http.Request) {
			nodes := []map[string]interface{}{
				{"id": "abc123", "name": "README.md", "type": "blob", "path": "README.md"},
				{"id": "def456", "name": "src", "type": "tree", "path": "src"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(nodes)
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

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := entities.Repository{Organization: "org", Name: "repo"}

		// when
		_, err := p.GetTags(context.Background(), repo)

		// then
		require.Error(t, err)
	})

	t.Run("should get tags from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, _ *http.Request) {
			tags := []map[string]interface{}{
				{"name": "v1.0.0"},
				{"name": "v2.0.0"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tags)
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
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("content"))
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
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
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

func TestCreateBranchWithChangesInternal(t *testing.T) {
	t.Parallel()

	t.Run("should return error when client is nil", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test", client: nil}
		repo := entities.Repository{Organization: "org", Name: "repo"}
		input := entities.BranchInput{BranchName: "feature"}

		// when
		err := p.CreateBranchWithChanges(context.Background(), repo, input)

		// then
		require.Error(t, err)
	})

	t.Run("should create branch with changes successfully", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				// Handle both branch creation and commit creation
				resp := map[string]interface{}{
					"name":       "feature",
					"short_id":   "abc123",
					"id":         "abc123def456",
					"title":      "Add new files",
					"created_at": "2024-01-01T00:00:00Z",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			}
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization:  "my-org",
			Name:          "my-repo",
			DefaultBranch: "refs/heads/main",
		}
		input := entities.BranchInput{
			BranchName:    "feature",
			BaseBranch:    "refs/heads/main",
			CommitMessage: "Add new files",
			Changes: []entities.FileChange{
				{Path: "/README.md", Content: "# Hello", ChangeType: "edit"},
			},
		}

		// when
		err := p.CreateBranchWithChanges(context.Background(), repo, input)

		// then
		require.NoError(t, err)
	})
}
