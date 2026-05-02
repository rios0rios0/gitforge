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

// commentStatusOptionCase describes one row of the WithThreadStatus
// option table reused by the two PostPullRequest* status tests below.
// Defined as a package-level type (not inline in each test func) so the
// shared `commentStatusOptionCases` helper can return it without
// duplicating the struct in two places — exactly the kind of
// "structurally identical table per test" that the SonarCloud quality
// gate flagged on the original PR.
type commentStatusOptionCase struct {
	name           string
	opts           []globalEntities.CommentOption
	expectedStatus string
}

// commentStatusOptionCases returns the canonical option-rows the two
// status tests below drive into PostPullRequestComment and
// PostPullRequestThreadComment respectively. Both methods take the
// same `...CommentOption` shape and route it through the same
// `ResolveCommentOptions` helper, so the option / expected-status
// mapping is identical — keeping it in one place means a future "add
// `byDesign` to the accepted set" or "rename WithThreadStatus" change
// only edits this one slice.
func commentStatusOptionCases() []commentStatusOptionCase {
	return []commentStatusOptionCase{
		{
			name:           "should default to active status when no options are provided",
			opts:           nil,
			expectedStatus: "active",
		},
		{
			name:           "should send closed status when WithThreadStatus closed is provided",
			opts:           []globalEntities.CommentOption{globalEntities.WithThreadStatus("closed")},
			expectedStatus: "closed",
		},
		{
			name:           "should send fixed status when WithThreadStatus fixed is provided",
			opts:           []globalEntities.CommentOption{globalEntities.WithThreadStatus("fixed")},
			expectedStatus: "fixed",
		},
	}
}

// captureThreadStatusFromPOST registers a single POST handler on `mux`
// that records the resulting JSON body's `status` value into
// `*captured`. Pulled out so both status tests share the same capture
// shape rather than each inlining the same `mux.HandleFunc(...)`
// boilerplate.
func captureThreadStatusFromPOST(t *testing.T, mux *http.ServeMux, endpoint string, capturedBody *map[string]any) {
	t.Helper()
	mux.HandleFunc("POST "+endpoint, func(w http.ResponseWriter, r *http.Request) {
		*capturedBody = captureThreadBody(t, r)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	})
}

func TestPostPullRequestCommentStatus(t *testing.T) {
	t.Parallel()

	const baseEndpoint = "/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/12"
	repo := globalEntities.Repository{
		Organization: "my-org",
		Project:      "my-project",
		ID:           "repo-1",
	}

	for _, tt := range commentStatusOptionCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var capturedBody map[string]any
			mux := http.NewServeMux()
			captureThreadStatusFromPOST(t, mux, baseEndpoint+"/threads", &capturedBody)
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			err := p.PostPullRequestComment(
				context.Background(), repo, 12, "informational marker", tt.opts...,
			)

			// then
			require.NoError(t, err)
			require.NotNil(t, capturedBody)
			assert.Equal(t, tt.expectedStatus, capturedBody["status"])
		})
	}
}

func TestPostPullRequestThreadCommentStatus(t *testing.T) {
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

	for _, tt := range commentStatusOptionCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var capturedBody map[string]any
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
			captureThreadStatusFromPOST(t, mux, baseEndpoint+"/threads", &capturedBody)
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			_, err := p.PostPullRequestThreadComment(
				context.Background(), repo, prID, "/README.md", 10, "comment body", tt.opts...,
			)

			// then
			require.NoError(t, err)
			require.NotNil(t, capturedBody)
			assert.Equal(t, tt.expectedStatus, capturedBody["status"])
		})
	}
}

func TestListOpenPullRequestsPropagatesIsDraft(t *testing.T) {
	t.Parallel()

	t.Run("should preserve isDraft on the resulting PullRequestDetail entries", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]any{
						{
							"pullRequestId": 1,
							"title":         "draft pr",
							"status":        "active",
							"isDraft":       true,
							"sourceRefName": "refs/heads/feat",
							"targetRefName": "refs/heads/main",
							"createdBy":     map[string]any{"displayName": "alice"},
						},
						{
							"pullRequestId": 2,
							"title":         "ready pr",
							"status":        "active",
							"isDraft":       false,
							"sourceRefName": "refs/heads/feat-2",
							"targetRefName": "refs/heads/main",
							"createdBy":     map[string]any{"displayName": "bob"},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		prs, err := p.ListOpenPullRequests(context.Background(), repo)

		// then
		require.NoError(t, err)
		require.Len(t, prs, 2, "drafts must be returned alongside ready PRs so the consumer can apply policy")
		assert.True(t, prs[0].IsDraft)
		assert.False(t, prs[1].IsDraft)
	})
}

const submitReviewerID = "00000000-0000-0000-0000-000000000abc"

// stubConnectionData wires the /{org}/_apis/connectionData endpoint on the given
// mux so SubmitPullRequestReview can resolve a reviewer ID. The handler counts
// how many times it is hit so callers can assert the [sync.Once] cache.
func stubConnectionData(t *testing.T, mux *http.ServeMux, hits *int) {
	t.Helper()
	stubConnectionDataFor(t, mux, "my-org", submitReviewerID, hits)
}

// stubConnectionDataFor wires the connectionData endpoint for a specific
// organization and reviewer ID. Useful for asserting per-org caching.
func stubConnectionDataFor(t *testing.T, mux *http.ServeMux, organization, reviewerID string, hits *int) {
	t.Helper()
	mux.HandleFunc("GET /"+organization+"/_apis/connectionData", func(w http.ResponseWriter, _ *http.Request) {
		if hits != nil {
			*hits++
		}
		resp := map[string]any{
			"authenticatedUser": map[string]any{"id": reviewerID},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func TestSubmitPullRequestReviewUsesPreviewAPIVersionForConnectionData(t *testing.T) {
	t.Parallel()

	// Pin the preview-suffix contract on the connectionData endpoint.
	// Captured live in dev pod logs at 2026-05-01 20:52 UTC where every
	// native review failed with `VssInvalidPreviewVersionException` because
	// the request used `api-version=7.0` (Azure DevOps marks
	// `/_apis/connectionData` as preview-only). A future "consolidate to a
	// single api-version" refactor would silently regress without this row.
	t.Run("should send api-version=7.0-preview.1 on /_apis/connectionData", func(t *testing.T) {
		t.Parallel()

		// given
		var capturedAPIVersion string
		mux := http.NewServeMux()
		mux.HandleFunc("GET /my-org/_apis/connectionData", func(w http.ResponseWriter, r *http.Request) {
			capturedAPIVersion = r.URL.Query().Get("api-version")
			resp := map[string]any{
				"authenticatedUser": map[string]any{"id": submitReviewerID},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc(
			"PUT /my-org/my-project/_apis/git/repositories/repo-1/pullrequests/4242/reviewers/"+submitReviewerID,
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		err := p.SubmitPullRequestReview(
			context.Background(), repo, 4242,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictApprove},
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, "7.0-preview.1", capturedAPIVersion,
			"connectionData is preview-only on Azure DevOps; the -preview flag must be supplied")
	})
}

func TestSubmitPullRequestReview(t *testing.T) {
	t.Parallel()

	const (
		prID         = 4242
		baseEndpoint = "/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/4242"
	)
	repo := globalEntities.Repository{
		Organization: "my-org",
		Project:      "my-project",
		ID:           "repo-1",
	}

	tests := []struct {
		name          string
		verdict       globalEntities.ReviewVerdict
		body          string
		expectedVote  int
		expectComment bool
	}{
		{
			name:          "should send vote 10 for ReviewVerdictApprove",
			verdict:       globalEntities.ReviewVerdictApprove,
			body:          "lgtm",
			expectedVote:  10,
			expectComment: true,
		},
		{
			name:          "should send vote -10 for ReviewVerdictRequestChanges",
			verdict:       globalEntities.ReviewVerdictRequestChanges,
			body:          "needs work",
			expectedVote:  -10,
			expectComment: true,
		},
		{
			name:          "should send vote -5 for ReviewVerdictWaitingForAuthor",
			verdict:       globalEntities.ReviewVerdictWaitingForAuthor,
			body:          "ping author",
			expectedVote:  -5,
			expectComment: true,
		},
		{
			name:          "should send vote 0 for ReviewVerdictComment with body",
			verdict:       globalEntities.ReviewVerdictComment,
			body:          "FYI",
			expectedVote:  0,
			expectComment: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var capturedVote map[string]any
			var commentHits int
			mux := http.NewServeMux()
			stubConnectionData(t, mux, nil)
			mux.HandleFunc(
				"POST "+baseEndpoint+"/threads",
				func(w http.ResponseWriter, _ *http.Request) {
					commentHits++
					resp := map[string]any{"id": 1}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
				},
			)
			mux.HandleFunc(
				"PUT "+baseEndpoint+"/reviewers/"+submitReviewerID,
				func(w http.ResponseWriter, r *http.Request) {
					capturedVote = captureThreadBody(t, r)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				},
			)
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			err := p.SubmitPullRequestReview(
				context.Background(), repo, prID,
				globalEntities.ReviewSubmission{Verdict: tt.verdict, Body: tt.body},
			)

			// then
			require.NoError(t, err)
			require.NotNil(t, capturedVote)
			assert.InDelta(t, float64(tt.expectedVote), capturedVote["vote"], 0)
			assert.Equal(t, submitReviewerID, capturedVote["id"])
			if tt.expectComment {
				assert.Equal(t, 1, commentHits, "summary body should be posted as a PR comment first")
			} else {
				assert.Equal(t, 0, commentHits)
			}
		})
	}
}

func TestSubmitPullRequestReviewSkipsEmptyComment(t *testing.T) {
	t.Parallel()

	t.Run("should skip the API entirely for ReviewVerdictComment with empty body", func(t *testing.T) {
		t.Parallel()

		// given
		var connectionHits int
		mux := http.NewServeMux()
		stubConnectionData(t, mux, &connectionHits)
		// any unexpected POST/PUT will fail the test
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		err := p.SubmitPullRequestReview(
			context.Background(), repo, 4242,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictComment},
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, 0, connectionHits, "skipped path must not even resolve the reviewer ID")
	})
}

func TestSubmitPullRequestReviewCachesReviewerID(t *testing.T) {
	t.Parallel()

	t.Run("should call connectionData only once across multiple submissions", func(t *testing.T) {
		t.Parallel()

		// given
		const baseEndpoint = "/my-org/my-project/_apis/git/repositories/repo-1/pullrequests/4242"
		var connectionHits int
		mux := http.NewServeMux()
		stubConnectionData(t, mux, &connectionHits)
		mux.HandleFunc(
			"PUT "+baseEndpoint+"/reviewers/"+submitReviewerID,
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		err1 := p.SubmitPullRequestReview(
			context.Background(), repo, 4242,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictApprove},
		)
		err2 := p.SubmitPullRequestReview(
			context.Background(), repo, 4242,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictApprove},
		)

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, 1, connectionHits, "reviewer ID lookup must be memoised by sync.Once")
	})
}

func TestSubmitPullRequestReviewCachesReviewerIDPerOrganization(t *testing.T) {
	t.Parallel()

	t.Run("should resolve reviewer ID independently for each organization", func(t *testing.T) {
		t.Parallel()

		// given
		const (
			orgA      = "org-a"
			orgB      = "org-b"
			reviewerA = "00000000-0000-0000-0000-00000000aaaa"
			reviewerB = "00000000-0000-0000-0000-00000000bbbb"
			project   = "my-project"
			repoID    = "repo-1"
			prID      = 4242
		)
		var hitsA, hitsB int
		mux := http.NewServeMux()
		stubConnectionDataFor(t, mux, orgA, reviewerA, &hitsA)
		stubConnectionDataFor(t, mux, orgB, reviewerB, &hitsB)
		mux.HandleFunc(
			"PUT /"+orgA+"/"+project+"/_apis/git/repositories/"+repoID+"/pullrequests/4242/reviewers/"+reviewerA,
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			},
		)
		mux.HandleFunc(
			"PUT /"+orgB+"/"+project+"/_apis/git/repositories/"+repoID+"/pullrequests/4242/reviewers/"+reviewerB,
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repoA := globalEntities.Repository{Organization: orgA, Project: project, ID: repoID}
		repoB := globalEntities.Repository{Organization: orgB, Project: project, ID: repoID}

		// when
		errA := p.SubmitPullRequestReview(
			context.Background(), repoA, prID,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictApprove},
		)
		errB := p.SubmitPullRequestReview(
			context.Background(), repoB, prID,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictApprove},
		)
		errA2 := p.SubmitPullRequestReview(
			context.Background(), repoA, prID,
			globalEntities.ReviewSubmission{Verdict: globalEntities.ReviewVerdictApprove},
		)

		// then
		require.NoError(t, errA)
		require.NoError(t, errB)
		require.NoError(t, errA2)
		assert.Equal(t, 1, hitsA, "reviewer ID for org-a should be looked up exactly once")
		assert.Equal(t, 1, hitsB, "reviewer ID for org-b should be looked up exactly once")
	})
}

func TestSubmitPullRequestReviewRejectsUnknownVerdict(t *testing.T) {
	t.Parallel()

	t.Run("should return error for unrecognised verdict", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test"}
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		err := p.SubmitPullRequestReview(
			context.Background(), repo, 4242,
			globalEntities.ReviewSubmission{Verdict: "made-up"},
		)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported review verdict")
	})
}

func TestListPullRequestComments(t *testing.T) {
	t.Parallel()

	t.Run("should flatten threads into comments and drop system entries", func(t *testing.T) {
		t.Parallel()

		// given: one PR-wide thread (no threadContext), one inline thread
		// (with filePath + rightFileStart.line), and one system thread
		// (vote-changed) that must be dropped — both consumers (review-
		// once gate + comment dedup) want only human + bot text.
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests/4242/threads",
			func(w http.ResponseWriter, _ *http.Request) {
				resp := map[string]any{
					"value": []map[string]any{
						{
							"id": 9001,
							"comments": []map[string]any{
								{
									"id":              1,
									"parentCommentId": 0,
									"content":         "✅ **Code Guru review complete.**",
									"commentType":     "text",
									"author": map[string]any{
										"displayName": "code-guru",
										"uniqueName":  "code-guru@bot",
									},
								},
							},
						},
						{
							"id": 9002,
							"threadContext": map[string]any{
								"filePath":       "/internal/foo.go",
								"rightFileStart": map[string]any{"line": 42},
							},
							"comments": []map[string]any{
								{
									"id":              2,
									"parentCommentId": 0,
									"content":         "[high] this could be nil-checked",
									"commentType":     "text",
									"author": map[string]any{
										"displayName": "code-guru",
										"uniqueName":  "code-guru@bot",
									},
								},
								{
									"id":              3,
									"parentCommentId": 2,
									"content":         "addressed in next push",
									"commentType":     "text",
									"author": map[string]any{
										"displayName": "Felipe",
										"uniqueName":  "felipe@example",
									},
								},
							},
						},
						{
							"id": 9003,
							"comments": []map[string]any{
								{"id": 4, "content": "vote changed", "commentType": "system"},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		comments, err := p.ListPullRequestComments(context.Background(), repo, 4242)

		// then
		require.NoError(t, err)
		require.Len(t, comments, 3, "system thread must be dropped, leaving 1 + 2 text comments")
		assert.Equal(t, "✅ **Code Guru review complete.**", comments[0].Body)
		assert.Equal(t, "code-guru@bot", comments[0].Author)
		assert.Empty(t, comments[0].FilePath, "PR-wide thread has no file path")
		assert.Equal(t, "/internal/foo.go", comments[1].FilePath)
		assert.Equal(t, 42, comments[1].Line)
		assert.Equal(t, int64(9002), comments[1].ThreadID)
		assert.Equal(t, int64(2), comments[2].InReplyToID,
			"a reply must carry the parent comment ID via parentCommentId")
	})

	t.Run("should follow continuation token across pages", func(t *testing.T) {
		t.Parallel()

		// given: two-page response. The first page returns a continuation token
		// in the `X-Ms-Continuationtoken` header; without the loop the second
		// page's threads would be silently dropped, breaking dedup and the
		// "already reviewed" gate on busy PRs.
		mux := http.NewServeMux()
		mux.HandleFunc(
			"GET /my-org/my-project/_apis/git/repositories/repo-1/pullrequests/4242/threads",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("continuationToken") == "" {
					w.Header().Set(paginationHeader, "next-page-token")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"value": []map[string]any{
							{
								"id": 1001,
								"comments": []map[string]any{
									{
										"id":          11,
										"content":     "page-1 comment",
										"commentType": "text",
										"author": map[string]any{
											"uniqueName": "alice@example",
										},
									},
								},
							},
						},
					})
					return
				}
				assert.Equal(t, "next-page-token", r.URL.Query().Get("continuationToken"))
				_ = json.NewEncoder(w).Encode(map[string]any{
					"value": []map[string]any{
						{
							"id": 1002,
							"comments": []map[string]any{
								{
									"id":          22,
									"content":     "page-2 comment",
									"commentType": "text",
									"author": map[string]any{
										"uniqueName": "bob@example",
									},
								},
							},
						},
					},
				})
			},
		)
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Project: "my-project", ID: "repo-1"}

		// when
		comments, err := p.ListPullRequestComments(context.Background(), repo, 4242)

		// then
		require.NoError(t, err)
		require.Len(t, comments, 2,
			"both pages must be flattened; missing the loop drops the second page")
		assert.Equal(t, "page-1 comment", comments[0].Body)
		assert.Equal(t, "page-2 comment", comments[1].Body)
	})
}
