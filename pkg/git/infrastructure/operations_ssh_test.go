//go:build unit

package infrastructure

import (
	"net"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// acceptAllHostKeys is a stand-in callback: the tests assert which callback the
// fallback auth carries, never what it decides.
func acceptAllHostKeys(string, net.Addr, ssh.PublicKey) error { return nil }

// newInMemoryRepo returns an empty in-memory repository; the fallback-auth helpers
// only ever read its remote configuration, so no commit is needed.
func newInMemoryRepo(t *testing.T) *git.Repository {
	t.Helper()

	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)
	return repo
}

// newFakeAgentSocket returns the path of a listening Unix socket, which is all
// go-git's NewSSHAgentAuth needs to build the agent auth: the dial succeeds against
// the listener's backlog without anything speaking the agent protocol.
func newFakeAgentSocket(t *testing.T) string {
	t.Helper()

	sock := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return sock
}

// TestSetDefaultSSHHostKeyCallback is deliberately not parallel: it mutates the
// package-level fallback hook and the process environment.
func TestSetDefaultSSHHostKeyCallback(t *testing.T) {
	defer SetDefaultSSHHostKeyCallback(nil)
	agentSock := newFakeAgentSocket(t)

	t.Run("should leave the fallback auth nil when no callback is installed", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(nil)
		t.Setenv("SSH_AUTH_SOCK", agentSock)

		// when
		auth := defaultSSHAuth("ssh://git@github.com/org/repo.git")

		// then
		assert.Nil(t, auth)
	})

	t.Run("should build an agent auth carrying the installed callback", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", agentSock)

		// when
		auth := defaultSSHAuth("ssh://git@github.com/org/repo.git")

		// then
		require.NotNil(t, auth)
		publicKeys, ok := auth.(*gitssh.PublicKeysCallback)
		require.True(t, ok, "fallback auth should be an SSH agent PublicKeysCallback, got %T", auth)
		assert.Equal(t, "git", publicKeys.User)
		require.NotNil(t, publicKeys.HostKeyCallback)
		assert.Equal(
			t,
			reflect.ValueOf(ssh.HostKeyCallback(acceptAllHostKeys)).Pointer(),
			reflect.ValueOf(publicKeys.HostKeyCallback).Pointer(),
			"the fallback auth must carry the installed callback",
		)
	})

	t.Run("should keep the user the remote URL names", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", agentSock)

		// when
		auth := defaultSSHAuth("ssh://bob@gitlab.com/org/repo.git")

		// then
		require.NotNil(t, auth)
		publicKeys, ok := auth.(*gitssh.PublicKeysCallback)
		require.True(t, ok)
		assert.Equal(t, "bob", publicKeys.User)
	})

	t.Run("should not build a fallback auth for a non-SSH remote", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", agentSock)

		// when
		auth := defaultSSHAuth("https://github.com/org/repo.git")

		// then
		assert.Nil(t, auth)
	})

	t.Run("should not build a fallback auth when SSH_AUTH_SOCK is unset", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", "")

		// when
		auth := defaultSSHAuth("ssh://git@github.com/org/repo.git")

		// then
		assert.Nil(t, auth)
	})

	t.Run("should not build a fallback auth when the agent socket is dead", func(t *testing.T) {
		// given -- the WSL case: a relay that exited and left its socket path behind
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "dead.sock"))

		// when
		auth := defaultSSHAuth("ssh://git@github.com/org/repo.git")

		// then
		assert.Nil(t, auth)
	})
}

// TestDefaultSSHAuthForRepo is deliberately not parallel: it mutates the package-level
// fallback hook and the process environment.
func TestDefaultSSHAuthForRepo(t *testing.T) {
	defer SetDefaultSSHHostKeyCallback(nil)
	agentSock := newFakeAgentSocket(t)

	t.Run("should return nil when the repository has no origin", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", agentSock)
		repo := newInMemoryRepo(t)

		// when / then
		assert.Nil(t, defaultSSHAuthForRepo(repo))
	})

	t.Run("should build the fallback auth from the origin URL", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)
		t.Setenv("SSH_AUTH_SOCK", agentSock)
		repo := newInMemoryRepo(t)
		_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
			Name: "origin",
			URLs: []string{"ssh://git@github.com/org/repo.git"},
		})
		require.NoError(t, err)

		// when
		auth := defaultSSHAuthForRepo(repo)

		// then
		assert.NotNil(t, auth)
	})

	t.Run("should tolerate a nil repository", func(t *testing.T) {
		// given
		SetDefaultSSHHostKeyCallback(acceptAllHostKeys)

		// when / then
		assert.Nil(t, defaultSSHAuthForRepo(nil))
		assert.Nil(t, defaultSSHAuthForRemote(nil))
	})
}
