//go:build unit

package infrastructure_test

import (
	"testing"

	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitops "github.com/rios0rios0/gitforge/pkg/git/infrastructure"
)

func TestPushWithTransportDetection(t *testing.T) {
	t.Parallel()

	refSpec := gitcfg.RefSpec("refs/heads/master:refs/heads/master")

	t.Run("should attempt SSH push for git@ remote", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"git@github.com:org/repo.git"},
		})
		require.NoError(t, err)

		// when
		err = gitops.PushWithTransportDetection(repo, refSpec, nil)

		// then — error should come from SSH transport, not "no authentication methods"
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "no authentication methods provided")
		assert.NotContains(t, err.Error(), "unsupported remote URL scheme")
	})

	t.Run("should attempt SSH push for ssh:// remote", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"ssh://git@github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		err = gitops.PushWithTransportDetection(repo, refSpec, nil)

		// then — error should come from SSH transport, not auth or scheme
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "no authentication methods provided")
		assert.NotContains(t, err.Error(), "unsupported remote URL scheme")
	})

	t.Run("should return auth error for HTTPS remote with empty auth methods", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"https://github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		err = gitops.PushWithTransportDetection(repo, refSpec, []transport.AuthMethod{})

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no authentication methods provided")
	})

	t.Run("should return auth error for HTTP remote with empty auth methods", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"http://github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		err = gitops.PushWithTransportDetection(repo, refSpec, []transport.AuthMethod{})

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no authentication methods provided")
	})

	t.Run("should return error when origin remote not configured", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)

		// when
		err := gitops.PushWithTransportDetection(repo, refSpec, nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get origin remote")
	})

	t.Run("should return error for unsupported URL scheme", func(t *testing.T) {
		t.Parallel()

		// given
		repo := createInMemoryRepoWithCommit(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"ftp://github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		err = gitops.PushWithTransportDetection(repo, refSpec, nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported remote URL scheme")
	})
}
