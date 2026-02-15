package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/domain/entities"
	domainRepos "github.com/rios0rios0/gitforge/domain/repositories"
	gitops "github.com/rios0rios0/gitforge/infrastructure/git"
)

// mockAdapterFinder implements git.AdapterFinder for testing.
type mockAdapterFinder struct {
	adapterByServiceType domainRepos.LocalGitAuthProvider
	adapterByURL         domainRepos.LocalGitAuthProvider
}

func (m *mockAdapterFinder) GetAdapterByServiceType(_ entities.ServiceType) domainRepos.LocalGitAuthProvider {
	return m.adapterByServiceType
}

func (m *mockAdapterFinder) GetAdapterByURL(_ string) domainRepos.LocalGitAuthProvider {
	return m.adapterByURL
}

// mockLocalGitAuthProvider implements LocalGitAuthProvider for testing.
type mockLocalGitAuthProvider struct {
	serviceType entities.ServiceType
	authMethods []transport.AuthMethod
}

func (m *mockLocalGitAuthProvider) Name() string      { return "mock" }
func (m *mockLocalGitAuthProvider) AuthToken() string { return "mock-token" }
func (m *mockLocalGitAuthProvider) MatchesURL(_ string) bool {
	return true
}
func (m *mockLocalGitAuthProvider) CloneURL(_ entities.Repository) string { return "" }
func (m *mockLocalGitAuthProvider) DiscoverRepositories(
	_ context.Context, _ string,
) ([]entities.Repository, error) {
	return nil, nil
}
func (m *mockLocalGitAuthProvider) CreatePullRequest(
	_ context.Context, _ entities.Repository, _ entities.PullRequestInput,
) (*entities.PullRequest, error) {
	return nil, nil
}
func (m *mockLocalGitAuthProvider) PullRequestExists(
	_ context.Context, _ entities.Repository, _ string,
) (bool, error) {
	return false, nil
}
func (m *mockLocalGitAuthProvider) GetServiceType() entities.ServiceType { return m.serviceType }
func (m *mockLocalGitAuthProvider) PrepareCloneURL(url string) string    { return url }
func (m *mockLocalGitAuthProvider) ConfigureTransport()                  {}
func (m *mockLocalGitAuthProvider) GetAuthMethods(_ string) []transport.AuthMethod {
	return m.authMethods
}

type mockAuth struct{}

func (m *mockAuth) Name() string   { return "mock-auth" }
func (m *mockAuth) String() string { return "mock-auth" }

func TestSetAdapterFinder(t *testing.T) {
	t.Run("should set adapter finder without panic", func(t *testing.T) {
		// given
		finder := &mockAdapterFinder{}

		// when / then — should not panic
		assert.NotPanics(t, func() {
			gitops.SetAdapterFinder(finder)
		})
	})
}

func TestGetServiceTypeByURL(t *testing.T) {
	t.Run("should return BITBUCKET for bitbucket.org URL", func(t *testing.T) {
		// given
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByURL: nil})
		rawURL := "https://bitbucket.org/org/repo.git"

		// when
		result := gitops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, entities.BITBUCKET, result)
	})

	t.Run("should return CODECOMMIT for git-codecommit URL", func(t *testing.T) {
		// given
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByURL: nil})
		rawURL := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo"

		// when
		result := gitops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, entities.CODECOMMIT, result)
	})

	t.Run("should return UNKNOWN for unrecognized URL", func(t *testing.T) {
		// given
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByURL: nil})
		rawURL := "https://custom-git.example.com/repo.git"

		// when
		result := gitops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, entities.UNKNOWN, result)
	})

	t.Run("should return adapter service type when adapter matches", func(t *testing.T) {
		// given
		adapter := &mockLocalGitAuthProvider{serviceType: entities.GITHUB}
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByURL: adapter})
		rawURL := "https://github.com/org/repo.git"

		// when
		result := gitops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, entities.GITHUB, result)
	})
}

func TestGetAuthMethods(t *testing.T) {
	t.Run("should return error when no adapter found", func(t *testing.T) {
		// given
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByServiceType: nil})

		// when
		_, err := gitops.GetAuthMethods(entities.UNKNOWN, "user")

		// then
		require.Error(t, err)
	})

	t.Run("should return error when adapter returns no auth methods", func(t *testing.T) {
		// given
		adapter := &mockLocalGitAuthProvider{
			serviceType: entities.GITHUB,
			authMethods: nil,
		}
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByServiceType: adapter})

		// when
		_, err := gitops.GetAuthMethods(entities.GITHUB, "user")

		// then
		require.Error(t, err)
	})

	t.Run("should return auth methods when adapter provides them", func(t *testing.T) {
		// given
		adapter := &mockLocalGitAuthProvider{
			serviceType: entities.GITHUB,
			authMethods: []transport.AuthMethod{
				&mockAuth{},
			},
		}
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByServiceType: adapter})

		// when
		methods, err := gitops.GetAuthMethods(entities.GITHUB, "user")

		// then
		require.NoError(t, err)
		assert.Len(t, methods, 1)
	})
}

func TestOpenRepo(t *testing.T) {
	t.Parallel()

	t.Run("should return error when path does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		path := "/tmp/nonexistent_repo_xyz_12345"

		// when
		_, err := gitops.OpenRepo(path)

		// then
		require.Error(t, err)
	})
}

func TestCheckBranchExists(t *testing.T) {
	t.Parallel()

	t.Run("should return false when branch does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)

		// when
		exists, err := gitops.CheckBranchExists(repo, "nonexistent-branch")

		// then
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("should return true when branch exists", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)

		// when
		exists, err := gitops.CheckBranchExists(repo, "master")

		// then
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestGetAmountCommits(t *testing.T) {
	t.Parallel()

	t.Run("should return correct number of commits", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)

		// when
		count, err := gitops.GetAmountCommits(repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestGetRemoteRepoURL(t *testing.T) {
	t.Parallel()

	t.Run("should return error when no remote exists", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)

		// when
		_, err := gitops.GetRemoteRepoURL(repo)

		// then
		require.Error(t, err)
	})
}

func TestGetLatestTag(t *testing.T) {
	t.Parallel()

	t.Run("should return error when no tags and few commits", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)

		// when
		_, err := gitops.GetLatestTag(repo)

		// then
		require.Error(t, err)
	})

	t.Run("should return tag when tag exists", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		addTagToRepo(t, repo, "v1.0.0")

		// when
		latestTag, err := gitops.GetLatestTag(repo)

		// then
		require.NoError(t, err)
		require.NotNil(t, latestTag)
	})

	t.Run("should fall back to default tag when many commits and no tags", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithMultipleCommits(t, 6)

		// when
		latestTag, err := gitops.GetLatestTag(repo)

		// then
		require.NoError(t, err)
		require.NotNil(t, latestTag)
		assert.Equal(t, "0.1.0", latestTag.Tag.String())
	})
}

func TestCreateAndSwitchBranch(t *testing.T) {
	t.Parallel()

	t.Run("should create and switch to a new branch", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		headRef, err := repo.Head()
		require.NoError(t, err)

		// when
		err = gitops.CreateAndSwitchBranch(repo, wt, "feature-branch", headRef.Hash())

		// then
		require.NoError(t, err)
		exists, err := gitops.CheckBranchExists(repo, "feature-branch")
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestCheckoutBranch(t *testing.T) {
	t.Parallel()

	t.Run("should checkout existing branch", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		// when
		err = gitops.CheckoutBranch(wt, "master")

		// then
		require.NoError(t, err)
	})

	t.Run("should return error for non-existent branch", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		// when
		err = gitops.CheckoutBranch(wt, "nonexistent-branch")

		// then
		require.Error(t, err)
	})
}

func TestCommitChanges(t *testing.T) {
	t.Parallel()

	t.Run("should commit changes successfully", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		// Configure git user for commit
		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = repo.SetConfig(cfg)
		require.NoError(t, err)

		// Create a new file to commit
		newFile := filepath.Join(repoPath, "new-file.txt")
		err = os.WriteFile(newFile, []byte("new content"), 0o600)
		require.NoError(t, err)
		_, err = wt.Add("new-file.txt")
		require.NoError(t, err)

		// when
		hash, err := gitops.CommitChanges(wt, "test commit", nil, "Test User", "test@example.com")

		// then
		require.NoError(t, err)
		assert.NotEqual(t, plumbing.ZeroHash, hash)
	})
}

func TestGetRemoteServiceType(t *testing.T) {
	t.Run("should return service type for repo with remote", func(t *testing.T) {
		// given
		adapter := &mockLocalGitAuthProvider{serviceType: entities.GITHUB}
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByURL: adapter})

		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		serviceType, err := gitops.GetRemoteServiceType(repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, entities.GITHUB, serviceType)
	})

	t.Run("should return error when no remote URLs configured", func(t *testing.T) {
		// given
		gitops.SetAdapterFinder(&mockAdapterFinder{adapterByURL: nil})
		repo := createInMemoryRepoWithCommit(t)

		// when
		_, err := gitops.GetRemoteServiceType(repo)

		// then
		require.Error(t, err)
	})
}

func TestGetRemoteRepoURLWithRemote(t *testing.T) {
	t.Parallel()

	t.Run("should return remote URL when remote is configured", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		expectedURL := "https://github.com/org/repo.git"
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{expectedURL},
		})
		require.NoError(t, err)

		// when
		result, err := gitops.GetRemoteRepoURL(repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedURL, result)
	})
}

func TestOpenRepoSuccess(t *testing.T) {
	t.Parallel()

	t.Run("should open a valid git repository", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		_ = createFilesystemRepoWithCommit(t, repoPath)

		// when
		repo, err := gitops.OpenRepo(repoPath)

		// then
		require.NoError(t, err)
		require.NotNil(t, repo)
	})
}

// createInMemoryRepoWithCommit creates an in-memory git repo with one commit.
func createInMemoryRepoWithCommit(t *testing.T) *git.Repository {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Create an empty commit using low-level API
	sig := &object.Signature{
		Name:  "Test User",
		Email: "test@example.com",
		When:  time.Now(),
	}
	commitObj := &object.Commit{
		Author:    *sig,
		Committer: *sig,
		Message:   "Initial commit",
		TreeHash:  plumbing.ZeroHash,
	}

	obj := repo.Storer.NewEncodedObject()
	err = commitObj.Encode(obj)
	require.NoError(t, err)

	hash, err := repo.Storer.SetEncodedObject(obj)
	require.NoError(t, err)

	ref := plumbing.NewHashReference(plumbing.Master, hash)
	err = repo.Storer.SetReference(ref)
	require.NoError(t, err)

	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.Master)
	err = repo.Storer.SetReference(headRef)
	require.NoError(t, err)

	return repo
}

// createInMemoryRepoWithMultipleCommits creates an in-memory repo with N commits.
func createInMemoryRepoWithMultipleCommits(t *testing.T, n int) *git.Repository {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	var prevHash plumbing.Hash
	for i := range n {
		sig := &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now().Add(time.Duration(i) * time.Minute),
		}
		commitObj := &object.Commit{
			Author:    *sig,
			Committer: *sig,
			Message:   "Commit " + string(rune('A'+i)),
			TreeHash:  plumbing.ZeroHash,
		}
		if i > 0 {
			commitObj.ParentHashes = []plumbing.Hash{prevHash}
		}

		obj := repo.Storer.NewEncodedObject()
		err = commitObj.Encode(obj)
		require.NoError(t, err)

		prevHash, err = repo.Storer.SetEncodedObject(obj)
		require.NoError(t, err)
	}

	ref := plumbing.NewHashReference(plumbing.Master, prevHash)
	err = repo.Storer.SetReference(ref)
	require.NoError(t, err)

	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.Master)
	err = repo.Storer.SetReference(headRef)
	require.NoError(t, err)

	return repo
}

// addTagToRepo adds a lightweight tag to the in-memory repo.
func addTagToRepo(t *testing.T, repo *git.Repository, tagName string) {
	t.Helper()

	headRef, err := repo.Head()
	require.NoError(t, err)

	tagRef := plumbing.NewHashReference(
		plumbing.ReferenceName("refs/tags/"+tagName),
		headRef.Hash(),
	)
	err = repo.Storer.SetReference(tagRef)
	require.NoError(t, err)
}

// createFilesystemRepoWithCommit creates a git repo on disk with one commit.
func createFilesystemRepoWithCommit(t *testing.T, path string) *git.Repository {
	t.Helper()

	repo, err := git.PlainInit(path, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Create a file and commit
	testFile := filepath.Join(path, "README.md")
	err = os.WriteFile(testFile, []byte("# Test\n"), 0o600)
	require.NoError(t, err)

	_, err = wt.Add("README.md")
	require.NoError(t, err)

	_, err = wt.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	return repo
}
