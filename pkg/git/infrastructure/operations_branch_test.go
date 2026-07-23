//go:build unit

package infrastructure_test

import (
	"testing"

	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitops "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

// newRepoWithBareRemote builds a working repository whose origin is a real bare
// repository on disk, so listing and pushing exercise actual git plumbing.
func newRepoWithBareRemote(t *testing.T) *git.Repository {
	t.Helper()

	remotePath := t.TempDir()
	_, err := git.PlainInit(remotePath, true)
	require.NoError(t, err)

	repo := createFilesystemRepoWithCommit(t, t.TempDir())
	_, err = repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{remotePath},
		Fetch: []gitcfg.RefSpec{
			gitcfg.RefSpec("+refs/heads/*:refs/remotes/origin/*"),
		},
	})
	require.NoError(t, err)

	return repo
}

// pushBranch creates a branch at HEAD and publishes it to the origin remote.
func pushBranch(t *testing.T, repo *git.Repository, branch string) {
	t.Helper()

	head, err := repo.Head()
	require.NoError(t, err)

	require.NoError(t, repo.Storer.SetReference(
		plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branch), head.Hash()),
	))
	require.NoError(t, repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gitcfg.RefSpec{gitcfg.RefSpec("refs/heads/" + branch + ":refs/heads/" + branch)},
	}))
}

func TestListRemoteBranches(t *testing.T) {
	t.Parallel()

	t.Run("should list the branches published on the remote", func(t *testing.T) {
		t.Parallel()

		// given
		repo := newRepoWithBareRemote(t)
		pushBranch(t, repo, "chore/bump-1.0.0")
		pushBranch(t, repo, "feat/unrelated")

		// when
		branches, err := gitops.ListRemoteBranches(repo, nil)

		// then
		require.NoError(t, err)
		assert.Contains(t, branches, "chore/bump-1.0.0")
		assert.Contains(t, branches, "feat/unrelated")
	})

	t.Run("should return an error when the origin remote is missing", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createFilesystemRepoWithCommit(t, t.TempDir())

		// when
		_, err := gitops.ListRemoteBranches(repo, nil)

		// then
		require.Error(t, err)
	})
}

func TestDeleteRemoteBranch(t *testing.T) {
	t.Parallel()

	t.Run("should remove the branch from the remote", func(t *testing.T) {
		t.Parallel()

		// given
		repo := newRepoWithBareRemote(t)
		const branch = "chore/bump-1.0.0"
		pushBranch(t, repo, branch)
		pushBranch(t, repo, "feat/keep-me")

		published, err := gitops.ListRemoteBranches(repo, nil)
		require.NoError(t, err)
		require.Contains(t, published, branch)

		// when
		err = gitops.DeleteRemoteBranch(repo, branch, nil)

		// then the branch is gone from the remote and nothing else was touched,
		// which is what proves the ":refs/heads/<branch>" refspec and the origin
		// remote name are both right
		require.NoError(t, err)
		remaining, err := gitops.ListRemoteBranches(repo, nil)
		require.NoError(t, err)
		assert.NotContains(t, remaining, branch)
		assert.Contains(t, remaining, "feat/keep-me")
	})

	t.Run("should report an error when the branch is absent on the remote", func(t *testing.T) {
		t.Parallel()

		// given
		repo := newRepoWithBareRemote(t)

		// when
		err := gitops.DeleteRemoteBranch(repo, "chore/bump-9.9.9", nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chore/bump-9.9.9")
	})

	t.Run("should reject a remote whose URL scheme is not supported", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createFilesystemRepoWithCommit(t, t.TempDir())
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"nonsense://example.com/repo.git"},
		})
		require.NoError(t, err)

		// when
		err = gitops.DeleteRemoteBranch(repo, "chore/bump-1.0.0", nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported remote URL scheme")
	})
}

func TestDeleteLocalBranch(t *testing.T) {
	t.Parallel()

	t.Run("should remove the local branch so it stops looking like it exists", func(t *testing.T) {
		t.Parallel()

		// given a branch that was published and then deleted on the remote: go-git drops
		// the remote-tracking reference, but the local branch survives and keeps
		// CheckBranchExists answering "yes"
		repo := newRepoWithBareRemote(t)
		const branch = "chore/bump-1.0.0"
		pushBranch(t, repo, branch)
		require.NoError(t, repo.Push(&git.PushOptions{
			RemoteName: "origin",
			RefSpecs:   []gitcfg.RefSpec{gitcfg.RefSpec(":refs/heads/" + branch)},
		}))

		existsBefore, err := gitops.CheckBranchExists(repo, branch)
		require.NoError(t, err)
		require.True(t, existsBefore, "the local branch should still make the branch look present")

		// when
		err = gitops.DeleteLocalBranch(repo, branch)

		// then
		require.NoError(t, err)
		existsAfter, err := gitops.CheckBranchExists(repo, branch)
		require.NoError(t, err)
		assert.False(t, existsAfter, "the branch must be recreatable after cleanup")
	})

	t.Run("should do nothing when the branch does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createFilesystemRepoWithCommit(t, t.TempDir())

		// when
		err := gitops.DeleteLocalBranch(repo, "chore/bump-9.9.9")

		// then
		require.NoError(t, err)
	})

	t.Run("should refuse to delete the checked-out branch", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createFilesystemRepoWithCommit(t, t.TempDir())
		head, err := repo.Head()
		require.NoError(t, err)

		// when deleting whatever branch HEAD currently points at
		err = gitops.DeleteLocalBranch(repo, head.Name().Short())

		// then the reference survives, because removing it would break the repository
		require.NoError(t, err)
		_, err = repo.Reference(head.Name(), false)
		assert.NoError(t, err)
	})
}
