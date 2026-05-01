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

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
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
			repos := []map[string]any{
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

	t.Run("should fall back to user repos when org listing returns 404", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /orgs/my-user/repos", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Not Found"}`))
		})
		mux.HandleFunc("GET /users/my-user/repos", func(w http.ResponseWriter, _ *http.Request) {
			repos := []map[string]any{
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

	t.Run("should return error when org listing fails with non-404 status", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /orgs/my-org/repos", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)

		// when
		repos, err := p.DiscoverRepositories(context.Background(), "my-org")

		// then
		require.Error(t, err)
		assert.Nil(t, repos)
	})
}

func TestCreatePullRequestInternal(t *testing.T) {
	t.Parallel()

	t.Run("should create pull request successfully", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("POST /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			pr := map[string]any{
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
		repo := globalEntities.Repository{
			Organization: "my-org",
			Name:         "my-repo",
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
			prs := []map[string]any{
				{"number": 1, "title": "Existing PR"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(prs)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

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
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

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
			resp := map[string]any{
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
		repo := globalEntities.Repository{
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
			resp := map[string]any{
				"tree": []map[string]any{
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
		repo := globalEntities.Repository{
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
			resp := []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"sha": "abc"}},
				{"name": "v2.0.0", "commit": map[string]string{"sha": "def"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

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
			resp := map[string]any{
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
		repo := globalEntities.Repository{
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
		repo := globalEntities.Repository{
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

func TestCreateBranchWithChangesInternal(t *testing.T) {
	t.Parallel()

	t.Run("should create branch with changes successfully", func(t *testing.T) {
		t.Parallel()

		// given
		baseSHA := "abc123"
		treeSHA := "tree123"
		commitSHA := "commit123"

		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/git/ref/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]string{"sha": baseSHA, "type": "commit"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("GET /repos/my-org/my-repo/git/commits/"+baseSHA, func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"sha":  baseSHA,
				"tree": map[string]string{"sha": treeSHA},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("POST /repos/my-org/my-repo/git/trees", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"sha": "newtree123",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("POST /repos/my-org/my-repo/git/commits", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"sha": commitSHA,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("POST /repos/my-org/my-repo/git/refs", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"ref":    "refs/heads/feature",
				"object": map[string]string{"sha": commitSHA},
			}
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{
			Organization:  "my-org",
			Name:          "my-repo",
			DefaultBranch: "refs/heads/main",
		}
		input := globalEntities.BranchInput{
			BranchName:    "feature",
			BaseBranch:    "refs/heads/main",
			CommitMessage: "Add new files",
			Changes: []globalEntities.FileChange{
				{Path: "/README.md", Content: "# Hello", ChangeType: "edit"},
			},
		}

		// when
		err := p.CreateBranchWithChanges(context.Background(), repo, input)

		// then
		require.NoError(t, err)
	})
}

func TestPostPullRequestThreadCommentReturnsID(t *testing.T) {
	t.Parallel()

	t.Run("should return the new review ID as the thread ID", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("POST /repos/my-org/my-repo/pulls/7/reviews", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{"id": 9876, "state": "COMMENTED"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		threadID, err := p.PostPullRequestThreadComment(
			context.Background(), repo, 7, "README.md", 3, "nit",
		)

		// then
		require.NoError(t, err)
		assert.Equal(t, 9876, threadID)
	})
}

func TestUpdatePullRequestThreadStatus(t *testing.T) {
	t.Parallel()

	t.Run("should return ErrThreadStatusUpdateUnsupported", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test"}
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		err := p.UpdatePullRequestThreadStatus(context.Background(), repo, 7, 9, "fixed")

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrThreadStatusUpdateUnsupported)
	})
}

func TestGetPullRequestStatus(t *testing.T) {
	t.Parallel()

	t.Run("should return open when PR is open", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{"number": 7, "state": "open", "merged": false}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 7)

		// then
		require.NoError(t, err)
		assert.Equal(t, "open", status)
	})

	t.Run("should return merged when PR is closed and has a merged_at timestamp", func(t *testing.T) {
		t.Parallel()

		// given: real merged PRs always populate `merged_at` —
		// `GetPullRequestStatus` reads off that timestamp rather
		// than the `merged` boolean per Copilot review on PR #86
		// thread `PRRT_kwDORQWb3M5-6QA0` (the boolean is omitted
		// from some fixture/replay payloads, leading to false
		// negatives where a merged PR reports as `closed`).
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{
				"number":    7,
				"state":     "closed",
				"merged":    true,
				"merged_at": "2026-05-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 7)

		// then
		require.NoError(t, err)
		assert.Equal(t, "merged", status)
	})

	t.Run("should treat a merged_at-only payload (no `merged` boolean) as merged", func(t *testing.T) {
		// given: defensive — exactly the failure mode the Copilot
		// review flagged. A REST fixture / replay payload that
		// omits the `merged` boolean must still report `merged`
		// when `merged_at` is set, otherwise legitimately-merged
		// PRs would silently report as `closed`. Pin the contract
		// per PR #86 thread `PRRT_kwDORQWb3M5-6QA0`.
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls/9", func(w http.ResponseWriter, _ *http.Request) {
			// note: no `merged` field in the response
			resp := map[string]any{
				"number":    9,
				"state":     "closed",
				"merged_at": "2026-05-01T01:23:45Z",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 9)

		// then
		require.NoError(t, err)
		assert.Equal(t, "merged", status)
	})

	t.Run("should return closed when PR is closed without merge", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
			resp := map[string]any{"number": 7, "state": "closed", "merged": false}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 7)

		// then
		require.NoError(t, err)
		assert.Equal(t, "closed", status)
	})

	t.Run("should return error when API call fails", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		status, err := p.GetPullRequestStatus(context.Background(), repo, 7)

		// then
		require.Error(t, err)
		assert.Empty(t, status)
	})
}

// TestThreadStatusOptionIgnoredByGitHub pins the contract that
// WithThreadStatus is silently ignored across every GitHub-backed
// post-comment surface — neither the Issues comment endpoint nor the
// Pull Request review endpoint exposes a per-thread status field, so
// the option must be accepted (callers can write provider-agnostic
// code) without breaking the underlying request. One table-driven
// test covers both methods to keep the option-no-op contract pinned in
// a single place.
func TestThreadStatusOptionIgnoredByGitHub(t *testing.T) {
	t.Parallel()

	repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

	tests := []struct {
		name        string
		endpoint    string
		response    string
		responseObj map[string]any
		invoke      func(ctx context.Context, p *Provider) error
	}{
		{
			name:     "should accept WithThreadStatus on PostPullRequestComment and post normally",
			endpoint: "POST /repos/my-org/my-repo/issues/7/comments",
			response: `{"id":1,"body":"informational marker"}`,
			invoke: func(ctx context.Context, p *Provider) error {
				return p.PostPullRequestComment(
					ctx, repo, 7, "informational marker",
					globalEntities.WithThreadStatus("closed"),
				)
			},
		},
		{
			name:        "should accept WithThreadStatus on PostPullRequestThreadComment and create the review normally",
			endpoint:    "POST /repos/my-org/my-repo/pulls/7/reviews",
			responseObj: map[string]any{"id": 4242, "state": "COMMENTED"},
			invoke: func(ctx context.Context, p *Provider) error {
				_, err := p.PostPullRequestThreadComment(
					ctx, repo, 7, "README.md", 3, "nit",
					globalEntities.WithThreadStatus("closed"),
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var requestCount int
			mux := http.NewServeMux()
			mux.HandleFunc(tt.endpoint, func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				if tt.responseObj != nil {
					_ = json.NewEncoder(w).Encode(tt.responseObj)
					return
				}
				_, _ = w.Write([]byte(tt.response))
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)

			// when
			err := tt.invoke(context.Background(), p)

			// then
			require.NoError(t, err)
			assert.Equal(t, 1, requestCount)
		})
	}
}
