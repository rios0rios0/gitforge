package infrastructure_test

import (
	"os"
	"os/exec"
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

	gitops "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	signingInfra "github.com/rios0rios0/gitforge/v4/pkg/signing/infrastructure"
	"github.com/rios0rios0/gitforge/v4/test/builders"
	"github.com/rios0rios0/gitforge/v4/test/doubles"
)

func TestNewGitOperations(t *testing.T) {
	t.Parallel()

	t.Run("should create GitOperations without panic", func(t *testing.T) {
		t.Parallel()

		// given
		finder := builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub)

		// when / then — should not panic
		assert.NotPanics(t, func() {
			gitops.NewGitOperations(finder)
		})
	})
}

func TestGetServiceTypeByURL(t *testing.T) {
	t.Parallel()

	t.Run("should return BITBUCKET for bitbucket.org URL", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))
		rawURL := "https://bitbucket.org/org/repo.git"

		// when
		result := ops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, globalEntities.BITBUCKET, result)
	})

	t.Run("should return CODECOMMIT for git-codecommit URL", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))
		rawURL := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo"

		// when
		result := ops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, globalEntities.CODECOMMIT, result)
	})

	t.Run("should return UNKNOWN for unrecognized URL", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))
		rawURL := "https://custom-git.example.com/repo.git"

		// when
		result := ops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, globalEntities.UNKNOWN, result)
	})

	t.Run("should return adapter service type when adapter matches", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().WithServiceType(globalEntities.GITHUB).Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(
			builders.NewAdapterFinderStubBuilder().WithAdapterByURL(adapter).Build().(*doubles.AdapterFinderStub),
		)
		rawURL := "https://github.com/org/repo.git"

		// when
		result := ops.GetServiceTypeByURL(rawURL)

		// then
		assert.Equal(t, globalEntities.GITHUB, result)
	})
}

func TestGetAuthMethods(t *testing.T) {
	t.Parallel()

	t.Run("should return error when no adapter found", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))

		// when
		_, err := ops.GetAuthMethods(globalEntities.UNKNOWN, "user")

		// then
		require.Error(t, err)
	})

	t.Run("should return error when adapter returns no auth methods", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().WithServiceType(globalEntities.GITHUB).Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(
			builders.NewAdapterFinderStubBuilder().WithAdapterByServiceType(adapter).Build().(*doubles.AdapterFinderStub),
		)

		// when
		_, err := ops.GetAuthMethods(globalEntities.GITHUB, "user")

		// then
		require.Error(t, err)
	})

	t.Run("should return auth methods when adapter provides them", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().
			WithServiceType(globalEntities.GITHUB).
			WithAuthMethods([]transport.AuthMethod{&doubles.AuthStub{}}).
			Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(
			builders.NewAdapterFinderStubBuilder().WithAdapterByServiceType(adapter).Build().(*doubles.AdapterFinderStub),
		)

		// when
		methods, err := ops.GetAuthMethods(globalEntities.GITHUB, "user")

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

	t.Run("should return the highest version rather than the tag listed last", func(t *testing.T) {
		t.Parallel()

		// given two versions whose names sort the other way round as text, on disk, where
		// tags are listed in the order of their names
		repo := createFilesystemRepoWithCommit(t, t.TempDir())
		addTagToRepo(t, repo, "1.9.0")
		addTagToRepo(t, repo, "1.10.0")

		// when
		latestTag, err := gitops.GetLatestTag(repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, "1.10.0", latestTag.Tag.String())
	})

	t.Run("should ignore tags that are not semantic versions", func(t *testing.T) {
		t.Parallel()

		// given a fork carrying its upstream's four-part tags beside its own, on disk, where
		// the four-part one is listed last
		repo := createFilesystemRepoWithCommit(t, t.TempDir())
		addTagToRepo(t, repo, "3.8.0.0")
		addTagToRepo(t, repo, "nightly")
		addTagToRepo(t, repo, "3.8.0-1")

		// when
		latestTag, err := gitops.GetLatestTag(repo)

		// then the version is never nil, which callers used to print as `<nil>`
		require.NoError(t, err)
		require.NotNil(t, latestTag.Tag)
		assert.Equal(t, "3.8.0-1", latestTag.Tag.String())
	})

	t.Run("should treat a repository whose tags are not versions like one without tags", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		addTagToRepo(t, repo, "3.8.0.0")

		// when
		latestTag, err := gitops.GetLatestTag(repo)

		// then
		require.ErrorIs(t, err, gitops.ErrNoTagsFound)
		assert.Nil(t, latestTag)
	})

	t.Run("should read the date of the commit an annotated tag points to", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		head, err := repo.Head()
		require.NoError(t, err)
		commit, err := repo.CommitObject(head.Hash())
		require.NoError(t, err)
		_, err = repo.CreateTag("v2.0.0", head.Hash(), &git.CreateTagOptions{
			Tagger:  &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Now().Add(time.Hour)},
			Message: "release 2.0.0",
		})
		require.NoError(t, err)

		// when
		latestTag, err := gitops.GetLatestTag(repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", latestTag.Tag.String())
		assert.True(t, commit.Committer.When.Equal(latestTag.Date), "the date is the commit's, not the tagger's")
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

	t.Run("should leave every local change and every untracked and ignored file in place", func(t *testing.T) {
		t.Parallel()

		// given an operator's checkout holding work no commit knows about
		repoPath := t.TempDir()
		repo, wt := createFilesystemRepoWithIgnoredFile(t, repoPath)
		headRef, err := repo.Head()
		require.NoError(t, err)
		files := writeUncommittedWork(t, repoPath, wt)

		// when
		err = gitops.CreateAndSwitchBranch(repo, wt, "feature-branch", headRef.Hash())

		// then the branch is current, and nothing in the working tree or the index moved
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)
		assert.Equal(t, plumbing.NewBranchReferenceName("feature-branch"), head.Name())
		assertUncommittedWorkKept(t, wt, files)
	})

	t.Run(
		"should refuse to switch to another commit over local changes instead of discarding them",
		func(t *testing.T) {
			t.Parallel()

			// given a branch to create at an older commit, and a local edit in the way
			repoPath := t.TempDir()
			repo, wt := createFilesystemRepoWithIgnoredFile(t, repoPath)
			older, err := repo.Head()
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# Second\n"), 0o600))
			_, err = wt.Add("README.md")
			require.NoError(t, err)
			newer, err := wt.Commit("second commit", &git.CommitOptions{
				Author: &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
			})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# local edit\n"), 0o600))

			// when
			err = gitops.CreateAndSwitchBranch(repo, wt, "older-branch", older.Hash())

			// then the edit is kept, and HEAD and the branches are as they were before
			require.Error(t, err)
			content, readErr := os.ReadFile(filepath.Join(repoPath, "README.md"))
			require.NoError(t, readErr)
			assert.Equal(t, "# local edit\n", string(content))
			head, headErr := repo.Head()
			require.NoError(t, headErr)
			assert.Equal(t, older.Name(), head.Name(), "the refused switch still moved HEAD")
			assert.Equal(t, newer, head.Hash())
			exists, existsErr := gitops.CheckBranchExists(repo, "older-branch")
			require.NoError(t, existsErr)
			assert.False(t, exists, "the refused switch still created the branch")
		},
	)

	t.Run(
		"should move the index and the files along when the current branch is moved to another commit",
		func(t *testing.T) {
			t.Parallel()

			// given the current branch, to be moved back to the commit before .gitignore
			repoPath := t.TempDir()
			repo, wt := createFilesystemRepoWithIgnoredFile(t, repoPath)
			current, err := repo.Head()
			require.NoError(t, err)
			commit, err := repo.CommitObject(current.Hash())
			require.NoError(t, err)
			parent := commit.ParentHashes[0]

			// when
			err = gitops.CreateAndSwitchBranch(repo, wt, current.Name().Short(), parent)

			// then
			require.NoError(t, err)
			head, err := repo.Head()
			require.NoError(t, err)
			assert.Equal(t, current.Name(), head.Name())
			assert.Equal(t, parent, head.Hash())
			assert.NoFileExists(t, filepath.Join(repoPath, ".gitignore"), "a file of the old commit was left behind")
			status, err := wt.Status()
			require.NoError(t, err)
			assert.True(t, status.IsClean(), "the index or the files are not at the branch's commit:\n%s", status)
		},
	)

	t.Run(
		"should leave the current branch at its commit when local changes refuse moving it",
		func(t *testing.T) {
			t.Parallel()

			// given the current branch, to be moved back a commit, and a local edit in the way
			repoPath := t.TempDir()
			repo, wt := createFilesystemRepoWithIgnoredFile(t, repoPath)
			current, err := repo.Head()
			require.NoError(t, err)
			commit, err := repo.CommitObject(current.Hash())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# local edit\n"), 0o600))

			// when
			err = gitops.CreateAndSwitchBranch(repo, wt, current.Name().Short(), commit.ParentHashes[0])

			// then
			require.Error(t, err)
			head, headErr := repo.Head()
			require.NoError(t, headErr)
			assert.Equal(t, current.Name(), head.Name())
			assert.Equal(t, current.Hash(), head.Hash(), "the refused move still moved the branch")
			content, readErr := os.ReadFile(filepath.Join(repoPath, "README.md"))
			require.NoError(t, readErr)
			assert.Equal(t, "# local edit\n", string(content))
		},
	)
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

	t.Run("should keep untracked and ignored files when switching back", func(t *testing.T) {
		t.Parallel()

		// given an operator who switched to a new branch with work lying around
		repoPath := t.TempDir()
		repo, wt := createFilesystemRepoWithIgnoredFile(t, repoPath)
		headRef, err := repo.Head()
		require.NoError(t, err)
		require.NoError(t, gitops.CreateAndSwitchBranch(repo, wt, "feature-branch", headRef.Hash()))
		untracked := filepath.Join(repoPath, "notes.txt")
		ignored := filepath.Join(repoPath, ignoredFileName)
		require.NoError(t, os.WriteFile(untracked, []byte("notes\n"), 0o600))
		require.NoError(t, os.WriteFile(ignored, []byte("SETTING=fixture-value-placeholder\n"), 0o600))

		// when
		err = gitops.CheckoutBranch(wt, headRef.Name().Short())

		// then
		require.NoError(t, err)
		assert.FileExists(t, untracked)
		assert.FileExists(t, ignored)
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

	t.Run("should commit changes successfully with nil signer", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = repo.SetConfig(cfg)
		require.NoError(t, err)

		newFile := filepath.Join(repoPath, "new-file.txt")
		err = os.WriteFile(newFile, []byte("new content"), 0o600)
		require.NoError(t, err)
		_, err = wt.Add("new-file.txt")
		require.NoError(t, err)

		// when
		hash, err := gitops.CommitChanges(repo, wt, "test commit", nil, "Test User", "test@example.com")

		// then
		require.NoError(t, err)
		assert.NotEqual(t, plumbing.ZeroHash, hash)
	})

	t.Run("should commit changes with nil signer", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = repo.SetConfig(cfg)
		require.NoError(t, err)

		newFile := filepath.Join(repoPath, "file2.txt")
		err = os.WriteFile(newFile, []byte("content"), 0o600)
		require.NoError(t, err)
		_, err = wt.Add("file2.txt")
		require.NoError(t, err)

		// when
		hash, err := gitops.CommitChanges(
			repo,
			wt,
			"test commit",
			nil,
			"Test User",
			"test@example.com",
		)

		// then
		require.NoError(t, err)
		assert.NotEqual(t, plumbing.ZeroHash, hash)
	})

	t.Run("should include signed-off-by in commit message", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = repo.SetConfig(cfg)
		require.NoError(t, err)

		newFile := filepath.Join(repoPath, "file3.txt")
		err = os.WriteFile(newFile, []byte("content"), 0o600)
		require.NoError(t, err)
		_, err = wt.Add("file3.txt")
		require.NoError(t, err)

		// when
		hash, err := gitops.CommitChanges(repo, wt, "my commit msg", nil, "John Doe", "john@example.com")

		// then
		require.NoError(t, err)
		commitObj, err := repo.CommitObject(hash)
		require.NoError(t, err)
		assert.Contains(t, commitObj.Message, "Signed-off-by: John Doe <john@example.com>")
	})

	t.Run("should return error when SSH signer has invalid key", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = repo.SetConfig(cfg)
		require.NoError(t, err)

		newFile := filepath.Join(repoPath, "file4.txt")
		err = os.WriteFile(newFile, []byte("content"), 0o600)
		require.NoError(t, err)
		_, err = wt.Add("file4.txt")
		require.NoError(t, err)

		signer := signingInfra.NewSSHSigner("/tmp/nonexistent-key-xyz", "")

		// when
		hash, err := gitops.CommitChanges(repo, wt, "test commit", signer, "Test User", "test@example.com")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "commit signing failed")
		assert.Equal(t, plumbing.ZeroHash, hash)
	})

	t.Run("should sign commit with valid SSH key", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		cfg, err := repo.Config()
		require.NoError(t, err)
		cfg.User.Name = "Test User"
		cfg.User.Email = "test@example.com"
		err = repo.SetConfig(cfg)
		require.NoError(t, err)

		newFile := filepath.Join(repoPath, "file5.txt")
		err = os.WriteFile(newFile, []byte("content"), 0o600)
		require.NoError(t, err)
		_, err = wt.Add("file5.txt")
		require.NoError(t, err)

		// Generate a temporary SSH key for signing
		keyPath := filepath.Join(t.TempDir(), "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		require.NoError(t, cmd.Run())

		signer := signingInfra.NewSSHSigner(keyPath, "")

		// when
		hash, err := gitops.CommitChanges(repo, wt, "signed commit", signer, "Test User", "test@example.com")

		// then
		require.NoError(t, err)
		assert.NotEqual(t, plumbing.ZeroHash, hash)

		// Verify the commit has an SSH signature
		commitObj, err := repo.CommitObject(hash)
		require.NoError(t, err)
		assert.Contains(t, commitObj.PGPSignature, "-----BEGIN SSH SIGNATURE-----")
	})
}

func TestGetRemoteServiceType(t *testing.T) {
	t.Parallel()

	t.Run("should return service type for repo with remote", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().WithServiceType(globalEntities.GITHUB).Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(
			builders.NewAdapterFinderStubBuilder().WithAdapterByURL(adapter).Build().(*doubles.AdapterFinderStub),
		)

		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		serviceType, err := ops.GetRemoteServiceType(repo)

		// then
		require.NoError(t, err)
		assert.Equal(t, globalEntities.GITHUB, serviceType)
	})

	t.Run("should return error when no remote URLs configured", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))
		repo := createInMemoryRepoWithCommit(t)

		// when
		_, err := ops.GetRemoteServiceType(repo)

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

func TestWorktreeIsClean(t *testing.T) {
	t.Parallel()

	t.Run("should return true when worktree has no changes", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		// when
		clean, err := gitops.WorktreeIsClean(wt)

		// then
		require.NoError(t, err)
		assert.True(t, clean)
	})

	t.Run("should return false when worktree has untracked file", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(repoPath, "untracked.txt"), []byte("new"), 0o600)
		require.NoError(t, err)

		// when
		clean, err := gitops.WorktreeIsClean(wt)

		// then
		require.NoError(t, err)
		assert.False(t, clean)
	})

	t.Run("should return false when worktree has modified file", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("modified content"), 0o600)
		require.NoError(t, err)

		// when
		clean, err := gitops.WorktreeIsClean(wt)

		// then
		require.NoError(t, err)
		assert.False(t, clean)
	})
}

func TestStageAll(t *testing.T) {
	t.Parallel()

	t.Run("should stage new files", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(repoPath, "new-file.txt"), []byte("content"), 0o600)
		require.NoError(t, err)

		// when
		err = gitops.StageAll(wt)

		// then
		require.NoError(t, err)
		status, err := wt.Status()
		require.NoError(t, err)
		assert.Equal(t, git.Added, status.File("new-file.txt").Staging)
	})

	t.Run("should stage modified files", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("updated"), 0o600)
		require.NoError(t, err)

		// when
		err = gitops.StageAll(wt)

		// then
		require.NoError(t, err)
		status, err := wt.Status()
		require.NoError(t, err)
		assert.Equal(t, git.Modified, status.File("README.md").Staging)
	})

	t.Run("should stage deleted files", func(t *testing.T) {
		t.Parallel()

		// given
		repoPath := t.TempDir()
		repo := createFilesystemRepoWithCommit(t, repoPath)
		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = os.Remove(filepath.Join(repoPath, "README.md"))
		require.NoError(t, err)

		// when
		err = gitops.StageAll(wt)

		// then
		require.NoError(t, err)
		status, err := wt.Status()
		require.NoError(t, err)
		assert.Equal(t, git.Deleted, status.File("README.md").Staging)
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

// ignoredFileName is the file the .gitignore of createFilesystemRepoWithIgnoredFile ignores.
const ignoredFileName = "local.env"

// createFilesystemRepoWithIgnoredFile creates a git repo on disk with two commits, the
// second adding a .gitignore that ignores ignoredFileName, and returns it with its worktree.
func createFilesystemRepoWithIgnoredFile(t *testing.T, path string) (*git.Repository, *git.Worktree) {
	t.Helper()

	repo := createFilesystemRepoWithCommit(t, path)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(path, ".gitignore"), []byte(ignoredFileName+"\n"), 0o600))
	_, err = wt.Add(".gitignore")
	require.NoError(t, err)
	_, err = wt.Commit("ignored "+ignoredFileName, &git.CommitOptions{
		Author: &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)
	return repo, wt
}

// uncommittedWork names the files writeUncommittedWork leaves in a checkout.
type uncommittedWork struct {
	edited, staged, untracked, ignored string
}

// writeUncommittedWork leaves one of each kind of work a commit does not hold: an edit
// to a tracked file, a staged new file, an untracked file and an ignored one.
func writeUncommittedWork(t *testing.T, path string, wt *git.Worktree) uncommittedWork {
	t.Helper()

	files := uncommittedWork{
		edited:    filepath.Join(path, "README.md"),
		staged:    filepath.Join(path, "staged.txt"),
		untracked: filepath.Join(path, "notes.txt"),
		ignored:   filepath.Join(path, ignoredFileName),
	}
	require.NoError(t, os.WriteFile(files.edited, []byte("# local edit\n"), 0o600))
	require.NoError(t, os.WriteFile(files.staged, []byte("staged\n"), 0o600))
	_, err := wt.Add("staged.txt")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(files.untracked, []byte("notes\n"), 0o600))
	require.NoError(t, os.WriteFile(files.ignored, []byte("SETTING=fixture-value-placeholder\n"), 0o600))
	return files
}

// assertUncommittedWorkKept asserts every file writeUncommittedWork left is still
// there, the edit unreverted and the staged file still staged.
func assertUncommittedWorkKept(t *testing.T, wt *git.Worktree, files uncommittedWork) {
	t.Helper()

	content, err := os.ReadFile(files.edited)
	require.NoError(t, err)
	assert.Equal(t, "# local edit\n", string(content), "the edit to a tracked file was reverted")
	assert.FileExists(t, files.staged)
	assert.FileExists(t, files.untracked)
	assert.FileExists(t, files.ignored)

	status, err := wt.Status()
	require.NoError(t, err)
	assert.Equal(t, git.Added, status.File("staged.txt").Staging, "the staged file should still be staged")
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

func TestCloneRepo(t *testing.T) {
	t.Parallel()

	t.Run("should return error when no auth methods provided", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))
		dir := t.TempDir()

		// when
		_, err := ops.CloneRepo("https://unknown-host.example.com/repo.git", dir, nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no authentication methods provided")
	})

	t.Run("should not leak credentials in error messages when URL contains userinfo", func(t *testing.T) {
		t.Parallel()

		// given
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().Build().(*doubles.AdapterFinderStub))
		dir := t.TempDir()
		urlWithCreds := "https://x-access-token:$test-token@github.com/org/repo.git"

		// when
		_, err := ops.CloneRepo(urlWithCreds, dir, nil)

		// then
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "$test-token")
		assert.NotContains(t, err.Error(), "x-access-token")
		assert.Contains(t, err.Error(), "github.com/org/repo.git")
	})

	t.Run("should not leak credentials in error messages when all auth methods fail", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().WithServiceType(globalEntities.GITHUB).Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().
			WithAdapterByURL(adapter).
			WithAdapterByServiceType(adapter).
			Build().(*doubles.AdapterFinderStub))
		dir := t.TempDir()
		urlWithCreds := "https://x-access-token:$test-token@github.com/org/repo.git"
		authMethods := []transport.AuthMethod{&doubles.AuthStub{}}

		// when
		_, err := ops.CloneRepo(urlWithCreds, dir, authMethods)

		// then
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "$test-token")
		assert.NotContains(t, err.Error(), "x-access-token")
	})

	t.Run("should return error when all auth methods fail", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().WithServiceType(globalEntities.GITHUB).Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().
			WithAdapterByURL(adapter).
			WithAdapterByServiceType(adapter).
			Build().(*doubles.AdapterFinderStub))
		dir := t.TempDir()
		authMethods := []transport.AuthMethod{&doubles.AuthStub{}}

		// when
		_, err := ops.CloneRepo("https://github.com/nonexistent/repo.git", dir, authMethods)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to clone")
	})

	t.Run("should prepare clone URL via adapter when adapter is found", func(t *testing.T) {
		t.Parallel()

		// given
		adapter := builders.NewForgeProviderStubBuilder().WithServiceType(globalEntities.GITHUB).Build().(*doubles.ForgeProviderStub)
		ops := gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().
			WithAdapterByURL(adapter).
			WithAdapterByServiceType(adapter).
			Build().(*doubles.AdapterFinderStub))
		dir := t.TempDir()
		authMethods := []transport.AuthMethod{&doubles.AuthStub{}}

		// when — clone will fail because the URL is not a real remote,
		// but the adapter's PrepareCloneURL and ConfigureTransport are called
		_, err := ops.CloneRepo("https://github.com/org/repo.git", dir, authMethods)

		// then — error from go-git because the URL is not reachable
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to clone")
	})
}
