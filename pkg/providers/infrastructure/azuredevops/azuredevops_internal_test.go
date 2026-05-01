package azuredevops

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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

func TestResolveRepoIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("should return ID when ID is set", func(t *testing.T) {
		t.Parallel()

		// given
		repo := globalEntities.Repository{ID: "repo-uuid-123", Name: "my-repo"}

		// when
		result := resolveRepoIdentifier(repo)

		// then
		assert.Equal(t, "repo-uuid-123", result)
	})

	t.Run("should fall back to Name when ID is empty", func(t *testing.T) {
		t.Parallel()

		// given
		repo := globalEntities.Repository{ID: "", Name: "my-repo"}

		// when
		result := resolveRepoIdentifier(repo)

		// then
		assert.Equal(t, "my-repo", result)
	})

	t.Run("should URL-encode Name when falling back to Name with special characters", func(t *testing.T) {
		t.Parallel()

		// given
		repo := globalEntities.Repository{ID: "", Name: "my repo with spaces"}

		// when
		result := resolveRepoIdentifier(repo)

		// then
		assert.Equal(t, "my%20repo%20with%20spaces", result)
	})
}

func TestEnsureRefsPrefix(t *testing.T) {
	t.Parallel()

	t.Run("should prepend refs/heads/ when missing", func(t *testing.T) {
		t.Parallel()

		// given
		branch := "main"

		// when
		result := ensureRefsPrefix(branch)

		// then
		assert.Equal(t, "refs/heads/main", result)
	})

	t.Run("should keep refs/heads/ when already present", func(t *testing.T) {
		t.Parallel()

		// given
		branch := "refs/heads/feature"

		// when
		result := ensureRefsPrefix(branch)

		// then
		assert.Equal(t, "refs/heads/feature", result)
	})

	t.Run("should keep refs/tags/ when present", func(t *testing.T) {
		t.Parallel()

		// given
		branch := "refs/tags/v1.0.0"

		// when
		result := ensureRefsPrefix(branch)

		// then
		assert.Equal(t, "refs/tags/v1.0.0", result)
	})
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
			resp := map[string]any{
				"value": []map[string]string{
					{"id": "proj-1", "name": "ProjectA"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("GET /my-org/proj-1/_apis/git/repositories", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"value": []map[string]any{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/items",
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("file content here"))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/items",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]any{
						{"objectId": "abc", "gitObjectType": "blob", "path": "/README.md"},
						{"objectId": "def", "gitObjectType": "tree", "path": "/src"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/refs",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]string{
						{"name": "refs/tags/v1.0.0"},
						{"name": "refs/tags/v2.0.0"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/items",
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("content"))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/items",
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]int{"count": 1}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]int{"count": 0}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
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
		mux.HandleFunc(
			"POST /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"pullRequestId": 42,
					"title":         "Test PR",
					"url":           "https://dev.azure.com/org/project/_git/repo/pullrequest/42",
					"status":        "active",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}
		input := globalEntities.PullRequestInput{
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

// captureThreadBody decodes the JSON body of a thread-create POST so the test can assert
// against the exact payload that was sent to ADO.
func captureThreadBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	defer func() { _ = r.Body.Close() }()
	raw, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return body
}

func TestPostPullRequestThreadCommentIterationContext(t *testing.T) {
	t.Parallel()

	const (
		prID         = 12105
		iterationID  = 7
		baseEndpoint = "/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12105"
	)
	repo := globalEntities.Repository{
		Organization: "my-org",
		Project:      "my-project",
		ID:           "repo-1",
	}

	t.Run("should include iterationContext and changeTrackingId when both lookups succeed", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedThreadBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]any{
						{"id": 1},
						{"id": iterationID},
						{"id": 5},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations/7/changes",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"changeEntries": []map[string]any{
						{
							"changeTrackingId": 42,
							"item":             map[string]string{"path": "/README.md"},
						},
						{
							"changeTrackingId": 99,
							"item":             map[string]string{"path": "/other.go"},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		mux.HandleFunc(
			"POST "+baseEndpoint+"/threads",
			func(w http.ResponseWriter, r *http.Request) {
				capturedThreadBody = captureThreadBody(t, r)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		_, err := p.PostPullRequestThreadComment(
			context.Background(), repo, prID, "/README.md", 10, "comment body",
		)

		// then
		require.NoError(t, err)
		require.NotNil(t, capturedThreadBody)
		ctxMap, ok := capturedThreadBody["pullRequestThreadContext"].(map[string]any)
		require.True(t, ok, "pullRequestThreadContext must be present")
		iterMap, ok := ctxMap["iterationContext"].(map[string]any)
		require.True(t, ok, "iterationContext must be present")
		assert.InDelta(t, float64(iterationID), iterMap["firstComparingIteration"], 0)
		assert.InDelta(t, float64(iterationID), iterMap["secondComparingIteration"], 0)
		assert.InDelta(t, float64(42), ctxMap["changeTrackingId"], 0)
	})

	t.Run("should post thread without iterationContext when iteration lookup fails", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedThreadBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations",
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"internal error"}`))
			},
		)
		mux.HandleFunc(
			"POST "+baseEndpoint+"/threads",
			func(w http.ResponseWriter, r *http.Request) {
				capturedThreadBody = captureThreadBody(t, r)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		_, err := p.PostPullRequestThreadComment(
			context.Background(), repo, prID, "README.md", 10, "comment body",
		)

		// then
		require.NoError(t, err)
		require.NotNil(t, capturedThreadBody)
		_, hasContext := capturedThreadBody["pullRequestThreadContext"]
		assert.False(t, hasContext, "pullRequestThreadContext must be absent when iteration lookup fails")
	})

	t.Run("should post thread with iterationContext but no changeTrackingId when no entry matches", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedThreadBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]any{{"id": iterationID}},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations/7/changes",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"changeEntries": []map[string]any{
						{
							"changeTrackingId": 1,
							"item":             map[string]string{"path": "/some-other-file.go"},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		mux.HandleFunc(
			"POST "+baseEndpoint+"/threads",
			func(w http.ResponseWriter, r *http.Request) {
				capturedThreadBody = captureThreadBody(t, r)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		_, err := p.PostPullRequestThreadComment(
			context.Background(), repo, prID, "README.md", 10, "comment body",
		)

		// then
		require.NoError(t, err)
		require.NotNil(t, capturedThreadBody)
		ctxMap, ok := capturedThreadBody["pullRequestThreadContext"].(map[string]any)
		require.True(t, ok, "pullRequestThreadContext must be present")
		_, hasIter := ctxMap["iterationContext"].(map[string]any)
		assert.True(t, hasIter, "iterationContext must be present")
		_, hasChangeID := ctxMap["changeTrackingId"]
		assert.False(t, hasChangeID, "changeTrackingId must be absent when no entry matches")
	})

	t.Run(
		"should post thread with iterationContext but no changeTrackingId when the changes lookup fails",
		func(t *testing.T) {
			// given: simulating a 5xx from `/iterations/{id}/changes`.
			// The iteration lookup succeeded so `iterationContext` must
			// still go on the thread, but `changeTrackingId` must be
			// absent because the lookup failed. Pinned per Copilot
			// review on PR #85 thread `PRRT_kwDORQWb3M5-6QSM`.
			t.Parallel()

			var capturedThreadBody map[string]any
			mux := http.NewServeMux()
			mux.HandleFunc(
				"GET "+baseEndpoint+"/iterations",
				func(w http.ResponseWriter, _ *http.Request) {
					resp := map[string]any{
						"value": []map[string]any{{"id": iterationID}},
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
				},
			)
			mux.HandleFunc(
				"GET "+baseEndpoint+"/iterations/7/changes",
				func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "internal", http.StatusInternalServerError)
				},
			)
			mux.HandleFunc(
				"POST "+baseEndpoint+"/threads",
				func(w http.ResponseWriter, r *http.Request) {
					capturedThreadBody = captureThreadBody(t, r)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":1}`))
				},
			)
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			_, err := p.PostPullRequestThreadComment(
				context.Background(), repo, prID, "README.md", 10, "comment body",
			)

			// then
			require.NoError(t, err)
			require.NotNil(t, capturedThreadBody)
			ctxMap, ok := capturedThreadBody["pullRequestThreadContext"].(map[string]any)
			require.True(t, ok, "pullRequestThreadContext must still be present (iteration lookup succeeded)")
			_, hasIter := ctxMap["iterationContext"].(map[string]any)
			assert.True(t, hasIter, "iterationContext must be present even when the changes lookup fails")
			_, hasChangeID := ctxMap["changeTrackingId"]
			assert.False(t, hasChangeID, "changeTrackingId must be absent when the changes lookup returns 5xx")
		},
	)

	t.Run("should match path when caller omits leading slash but ADO returns one", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedThreadBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]any{{"id": iterationID}},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		mux.HandleFunc(
			"GET "+baseEndpoint+"/iterations/7/changes",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"changeEntries": []map[string]any{
						{
							"changeTrackingId": 7,
							"item":             map[string]string{"path": "/README.md"},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		mux.HandleFunc(
			"POST "+baseEndpoint+"/threads",
			func(w http.ResponseWriter, r *http.Request) {
				capturedThreadBody = captureThreadBody(t, r)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":1}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when - caller passes the path without a leading slash
		_, err := p.PostPullRequestThreadComment(
			context.Background(), repo, prID, "README.md", 10, "comment body",
		)

		// then
		require.NoError(t, err)
		require.NotNil(t, capturedThreadBody)
		ctxMap, ok := capturedThreadBody["pullRequestThreadContext"].(map[string]any)
		require.True(t, ok)
		assert.InDelta(t, float64(7), ctxMap["changeTrackingId"], 0)
	})
}

func TestPostPullRequestThreadCommentReturnsID(t *testing.T) {
	t.Parallel()

	t.Run("should return the new thread ID from the response", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"POST /my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12/threads",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":4242}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		threadID, err := p.PostPullRequestThreadComment(
			context.Background(), repo, 12, "/README.md", 7, "looks good",
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, 4242, threadID)
	})
}

func TestUpdatePullRequestThreadStatus(t *testing.T) {
	t.Parallel()

	t.Run("should PATCH the thread with the given status", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedMethod string
		var capturedBody map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc(
			"/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12/threads/99",
			func(w http.ResponseWriter, r *http.Request) {
				capturedMethod = r.Method
				defer r.Body.Close()
				_ = json.NewDecoder(r.Body).Decode(&capturedBody)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":99,"status":"fixed"}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		err := p.UpdatePullRequestThreadStatus(context.Background(), repo, 12, 99, "fixed")

		// then
		require.NoError(t, err)
		assert.Equal(t, http.MethodPatch, capturedMethod)
		assert.Equal(t, "fixed", capturedBody["status"])
	})

	t.Run("should return error when API responds with failure", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12/threads/99",
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("server error"))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		err := p.UpdatePullRequestThreadStatus(context.Background(), repo, 12, 99, "fixed")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update pull request thread status")
	})
}

func TestGetPullRequestStatus(t *testing.T) {
	t.Parallel()

	t.Run("should return the status field from ADO response", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12",
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"pullRequestId":12,"status":"completed"}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 12)

		// then
		require.NoError(t, err)
		assert.Equal(t, "completed", status)
	})

	t.Run("should return error when API call fails", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12",
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("not found"))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization: "my-org",
			Project:      "my-project",
			ID:           "repo-1",
		}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 12)

		// then
		require.Error(t, err)
		assert.Empty(t, status)
	})
}
