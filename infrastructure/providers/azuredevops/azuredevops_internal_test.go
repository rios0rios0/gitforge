package azuredevops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
)

// redirectTransport redirects all requests to the given test server URL.
type redirectTransport struct {
	serverURL string
	inner     http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the scheme and host with the test server
	req.URL.Scheme = "http"
	req.URL.Host = t.serverURL[len("http://"):]
	return t.inner.RoundTrip(req)
}

// newTestProvider creates a Provider with http client redirected to the given test server.
func newTestProvider(t *testing.T, server *httptest.Server) *Provider {
	t.Helper()
	client := &http.Client{
		Transport: &redirectTransport{
			serverURL: server.URL,
			inner:     http.DefaultTransport,
		},
	}
	return &Provider{
		token:      "test-token",
		httpClient: client,
	}
}

func TestNormalizeOrgURL(t *testing.T) {
	t.Parallel()

	t.Run("should add https prefix when missing", func(t *testing.T) {
		t.Parallel()

		// given
		org := "my-org"

		// when
		result := normalizeOrgURL(org)

		// then
		assert.Equal(t, "https://dev.azure.com/my-org", result)
	})

	t.Run("should strip trailing slash", func(t *testing.T) {
		t.Parallel()

		// given
		org := "https://dev.azure.com/my-org/"

		// when
		result := normalizeOrgURL(org)

		// then
		assert.Equal(t, "https://dev.azure.com/my-org", result)
	})

	t.Run("should keep URL unchanged when already has https prefix", func(t *testing.T) {
		t.Parallel()

		// given
		org := "https://dev.azure.com/my-org"

		// when
		result := normalizeOrgURL(org)

		// then
		assert.Equal(t, "https://dev.azure.com/my-org", result)
	})
}

func TestBuildBaseURL(t *testing.T) {
	t.Parallel()

	t.Run("should build base URL with org name", func(t *testing.T) {
		t.Parallel()

		// given
		orgName := "my-org"

		// when
		result := buildBaseURL(orgName)

		// then
		assert.Equal(t, "https://dev.azure.com/my-org", result)
	})

	t.Run("should use only first part of slash-separated org", func(t *testing.T) {
		t.Parallel()

		// given
		orgName := "my-org/sub-part"

		// when
		result := buildBaseURL(orgName)

		// then
		assert.Equal(t, "https://dev.azure.com/my-org", result)
	})
}

func TestExtractOrgName(t *testing.T) {
	t.Parallel()

	t.Run("should extract org name from URL", func(t *testing.T) {
		t.Parallel()

		// given
		baseURL := "https://dev.azure.com/my-org"

		// when
		result := extractOrgName(baseURL)

		// then
		assert.Equal(t, "my-org", result)
	})

	t.Run("should return raw string when URL is invalid", func(t *testing.T) {
		t.Parallel()

		// given
		baseURL := "://invalid"

		// when
		result := extractOrgName(baseURL)

		// then
		assert.Equal(t, "://invalid", result)
	})
}

func TestConfigureTransport(t *testing.T) {
	t.Parallel()

	t.Run("should configure transport without panic", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test"}

		// when / then
		assert.NotPanics(t, func() {
			p.ConfigureTransport()
		})
	})
}

func TestDiscoverRepositories(t *testing.T) {
	t.Parallel()

	t.Run("should discover repositories from projects", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/projects", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"value": []map[string]string{
					{"id": "proj-1", "name": "ProjectA"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("GET /my-org/proj-1/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"value": []map[string]interface{}{
					{
						"id":            "repo-1",
						"name":          "RepoA",
						"remoteUrl":     "https://dev.azure.com/my-org/ProjectA/_git/RepoA",
						"sshUrl":        "git@ssh.dev.azure.com:v3/my-org/ProjectA/RepoA",
						"defaultBranch": "refs/heads/main",
						"project":       map[string]string{"name": "ProjectA"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.NoError(t, err)
		require.Len(t, repos, 1)
		assert.Equal(t, "RepoA", repos[0].Name)
		assert.Equal(t, providerName, repos[0].ProviderName)
	})
}

func TestGetFileContent(t *testing.T) {
	t.Parallel()

	t.Run("should get file content from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/items", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("file content here"))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		content, err := p.GetFileContent(context.Background(), repo, "README.md")

		// then
		require.NoError(t, err)
		assert.Equal(t, "file content here", content)
	})
}

func TestListFiles(t *testing.T) {
	t.Parallel()

	t.Run("should list files from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/items", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"value": []map[string]interface{}{
					{"objectId": "abc", "gitObjectType": "blob", "path": "/README.md"},
					{"objectId": "def", "gitObjectType": "tree", "path": "/src"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		files, err := p.ListFiles(context.Background(), repo, "")

		// then
		require.NoError(t, err)
		assert.Len(t, files, 2)
	})
}

func TestGetTags(t *testing.T) {
	t.Parallel()

	t.Run("should get tags from API", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/refs", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"value": []map[string]string{
					{"name": "refs/tags/v1.0.0"},
					{"name": "refs/tags/v2.0.0"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		tags, err := p.GetTags(context.Background(), repo)

		// then
		require.NoError(t, err)
		assert.Len(t, tags, 2)
		assert.Equal(t, "v2.0.0", tags[0])
		assert.Equal(t, "v1.0.0", tags[1])
	})
}

func TestHasFile(t *testing.T) {
	t.Parallel()

	t.Run("should return true when file exists", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/items", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("content"))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
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
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/items", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		result := p.HasFile(context.Background(), repo, "missing.md")

		// then
		assert.False(t, result)
	})
}

func TestPullRequestExists(t *testing.T) {
	t.Parallel()

	t.Run("should return true when PRs exist", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]int{"count": 1}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

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
		mux.HandleFunc("GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]int{"count": 0}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		exists, err := p.PullRequestExists(context.Background(), repo, "feature-branch")

		// then
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestCreatePullRequest(t *testing.T) {
	t.Parallel()

	t.Run("should create pull request successfully", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("POST /my-org/my-project/_apis/git/repositories/repo-1/pullrequests", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]interface{}{
				"pullRequestId": 42,
				"title":         "Test PR",
				"url":           "https://dev.azure.com/org/project/_git/repo/pullrequest/42",
				"status":        "active",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := entities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
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
	})
}

func TestDoRequest(t *testing.T) {
	t.Parallel()

	t.Run("should return error for non-2xx response", func(t *testing.T) {
		t.Parallel()

		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}))
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		_, err := p.doRequest(context.Background(), "https://dev.azure.com/org", http.MethodGet, "/test", nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API error")
	})

	t.Run("should send request body for POST", func(t *testing.T) {
		t.Parallel()

		// given
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": true}`))
		}))
		defer server.Close()

		p := newTestProvider(t, server)
		body := map[string]string{"key": "value"}

		// when
		resp, err := p.doRequest(context.Background(), "https://dev.azure.com/org", http.MethodPost, "/test", body)

		// then
		require.NoError(t, err)
		assert.NotEmpty(t, resp)
	})
}
