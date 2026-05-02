package github

import (
	"context"
	"encoding/json"
	"io"
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

func TestListOpenPullRequestsPropagatesIsDraft(t *testing.T) {
	t.Parallel()

	t.Run("should preserve draft on the resulting PullRequestDetail entries", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
			resp := []map[string]any{
				{
					"number":   1,
					"title":    "draft pr",
					"state":    "open",
					"draft":    true,
					"head":     map[string]any{"ref": "feat"},
					"base":     map[string]any{"ref": "main"},
					"user":     map[string]any{"login": "alice"},
					"html_url": "https://github.com/my-org/my-repo/pull/1",
				},
				{
					"number":   2,
					"title":    "ready pr",
					"state":    "open",
					"draft":    false,
					"head":     map[string]any{"ref": "feat-2"},
					"base":     map[string]any{"ref": "main"},
					"user":     map[string]any{"login": "bob"},
					"html_url": "https://github.com/my-org/my-repo/pull/2",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		prs, err := p.ListOpenPullRequests(context.Background(), repo)

		// then
		require.NoError(t, err)
		require.Len(t, prs, 2)
		assert.True(t, prs[0].IsDraft)
		assert.False(t, prs[1].IsDraft)
	})
}

// captureSubmitReviewEvent stands up a test server that records the `event`
// field sent on POST /pulls/:n/reviews and returns it for assertion. Shared by
// the verdict-mapping table tests so each row only owns its inputs/expectations.
func captureSubmitReviewEvent(
	t *testing.T,
	verdict globalEntities.ReviewVerdict,
	body string,
) (string, string, error) {
	t.Helper()

	var capturedEvent, capturedBody string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /repos/my-org/my-repo/pulls/7/reviews", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "failed to read request body")
		var payload struct {
			Event string `json:"event"`
			Body  string `json:"body"`
		}
		assert.NoError(t, json.Unmarshal(raw, &payload), "failed to unmarshal request body")
		capturedEvent = payload.Event
		capturedBody = payload.Body

		resp := map[string]any{"id": 1, "state": payload.Event}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := newTestProvider(t, server)
	repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}
	err := p.SubmitPullRequestReview(
		context.Background(), repo, 7,
		globalEntities.ReviewSubmission{Verdict: verdict, Body: body},
	)
	return capturedEvent, capturedBody, err
}

func TestSubmitPullRequestReview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		verdict       globalEntities.ReviewVerdict
		body          string
		expectedEvent string
	}{
		{
			name:          "should send APPROVE event for ReviewVerdictApprove",
			verdict:       globalEntities.ReviewVerdictApprove,
			body:          "looks good",
			expectedEvent: "APPROVE",
		},
		{
			name:          "should send REQUEST_CHANGES event for ReviewVerdictRequestChanges",
			verdict:       globalEntities.ReviewVerdictRequestChanges,
			body:          "needs work",
			expectedEvent: "REQUEST_CHANGES",
		},
		{
			name:          "should collapse ReviewVerdictWaitingForAuthor to COMMENT on GitHub (no native waiting state, soft signal mirrors ADO vote=-5)",
			verdict:       globalEntities.ReviewVerdictWaitingForAuthor,
			body:          "ping",
			expectedEvent: "COMMENT",
		},
		{
			name:          "should send COMMENT event for ReviewVerdictComment with non-empty body",
			verdict:       globalEntities.ReviewVerdictComment,
			body:          "FYI",
			expectedEvent: "COMMENT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given / when
			event, body, err := captureSubmitReviewEvent(t, tt.verdict, tt.body)

			// then
			require.NoError(t, err)
			assert.Equal(t, tt.expectedEvent, event)
			assert.Equal(t, tt.body, body)
		})
	}
}

func TestSubmitPullRequestReviewSkipsEmptyComment(t *testing.T) {
	t.Parallel()

	// Both verdicts collapse to GitHub event=COMMENT, so an empty body
	// must short-circuit before the API call (GitHub rejects empty COMMENT
	// reviews with 422 "Body is too short").
	tests := []struct {
		name    string
		verdict globalEntities.ReviewVerdict
	}{
		{
			name:    "should not call GitHub when verdict is comment and body is empty",
			verdict: globalEntities.ReviewVerdictComment,
		},
		{
			name:    "should not call GitHub when verdict is waiting_for_author and body is empty (collapses to COMMENT on GitHub)",
			verdict: globalEntities.ReviewVerdictWaitingForAuthor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var requestCount int
			mux := http.NewServeMux()
			mux.HandleFunc("POST /repos/my-org/my-repo/pulls/7/reviews", func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.WriteHeader(http.StatusInternalServerError)
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)
			repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

			// when
			err := p.SubmitPullRequestReview(
				context.Background(), repo, 7,
				globalEntities.ReviewSubmission{Verdict: tt.verdict},
			)

			// then
			require.NoError(t, err)
			assert.Equal(t, 0, requestCount)
		})
	}
}

func TestSubmitPullRequestReviewRejectsEmptyBodyForRequestChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		verdict globalEntities.ReviewVerdict
	}{
		{
			name:    "should reject ReviewVerdictRequestChanges with empty body before the API call",
			verdict: globalEntities.ReviewVerdictRequestChanges,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var requestCount int
			mux := http.NewServeMux()
			mux.HandleFunc("POST /repos/my-org/my-repo/pulls/7/reviews", func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.WriteHeader(http.StatusUnprocessableEntity)
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			p := newTestProvider(t, server)
			repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

			// when
			err := p.SubmitPullRequestReview(
				context.Background(), repo, 7,
				globalEntities.ReviewSubmission{Verdict: tt.verdict},
			)

			// then
			require.ErrorIs(t, err, ErrReviewBodyRequired)
			assert.Equal(t, 0, requestCount, "guard must short-circuit before the API call")
		})
	}
}

func TestSubmitPullRequestReviewSwallowsSelfReview(t *testing.T) {
	t.Parallel()

	t.Run("should swallow HTTP 422 when the body matches the self-review message", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("POST /repos/my-org/my-repo/pulls/7/reviews", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Can not approve your own pull request"}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		err := p.SubmitPullRequestReview(
			context.Background(), repo, 7,
			globalEntities.ReviewSubmission{
				Verdict: globalEntities.ReviewVerdictApprove,
				Body:    "lgtm",
			},
		)

		// then
		require.NoError(t, err)
	})

	t.Run("should return an error when HTTP 422 is not a self-review", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("POST /repos/my-org/my-repo/pulls/7/reviews", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(
				[]byte(
					`{"message":"Validation Failed","errors":[{"resource":"PullRequestReview","code":"missing_field"}]}`,
				),
			)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		err := p.SubmitPullRequestReview(
			context.Background(), repo, 7,
			globalEntities.ReviewSubmission{
				Verdict: globalEntities.ReviewVerdictApprove,
				Body:    "lgtm",
			},
		)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to submit pull request review")
	})
}

func TestSubmitPullRequestReviewRejectsUnknownVerdict(t *testing.T) {
	t.Parallel()

	t.Run("should return error for unrecognised verdict", func(t *testing.T) {
		t.Parallel()

		// given
		p := &Provider{token: "test"}
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		err := p.SubmitPullRequestReview(
			context.Background(), repo, 7,
			globalEntities.ReviewSubmission{Verdict: "made-up"},
		)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported review verdict")
	})
}

func TestListPullRequestComments(t *testing.T) {
	t.Parallel()

	t.Run("should merge issue comments and inline comments into a single list", func(t *testing.T) {
		t.Parallel()

		// given
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repos/my-org/my-repo/issues/7/comments", func(w http.ResponseWriter, _ *http.Request) {
			resp := []map[string]any{
				{"id": 100, "body": "PR-wide comment from a user", "user": map[string]any{"login": "alice"}},
				{
					"id":   101,
					"body": "✅ **Code Guru review complete.**",
					"user": map[string]any{"login": "code-guru[bot]"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		mux.HandleFunc("GET /repos/my-org/my-repo/pulls/7/comments", func(w http.ResponseWriter, _ *http.Request) {
			resp := []map[string]any{
				{
					"id":                     200,
					"pull_request_review_id": 5000,
					"path":                   "internal/foo.go",
					"line":                   42,
					"body":                   "[high] this could be nil-checked",
					"user":                   map[string]any{"login": "code-guru[bot]"},
				},
				{
					"id":                     201,
					"pull_request_review_id": 5000,
					"path":                   "internal/foo.go",
					"line":                   42,
					"body":                   "thanks, addressed in next push",
					"user":                   map[string]any{"login": "alice"},
					"in_reply_to_id":         200,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := newTestProvider(t, server)
		repo := globalEntities.Repository{Organization: "my-org", Name: "my-repo"}

		// when
		comments, err := p.ListPullRequestComments(context.Background(), repo, 7)

		// then
		require.NoError(t, err)
		require.Len(t, comments, 4)
		assert.Equal(t, "PR-wide comment from a user", comments[0].Body)
		assert.Equal(t, "alice", comments[0].Author)
		assert.Empty(t, comments[0].FilePath, "PR-wide comment must not carry a file path")
		assert.Zero(t, comments[0].Line)
		assert.Zero(t, comments[0].ThreadID)
		assert.Equal(t, "internal/foo.go", comments[2].FilePath)
		assert.Equal(t, 42, comments[2].Line)
		assert.Equal(t, int64(200), comments[2].ThreadID,
			"a top-level inline comment is its own thread root, so ThreadID = comment ID")
		assert.Zero(t, comments[2].InReplyToID, "top-level inline comment must have InReplyToID=0")
		assert.Equal(t, int64(200), comments[3].ThreadID,
			"a reply must share the thread root's ID — using pull_request_review_id "+
				"would merge unrelated threads from the same review submission")
		assert.Equal(t, int64(200), comments[3].InReplyToID,
			"a reply must carry the parent comment ID so a re-review pass can walk the thread")
	})
}
