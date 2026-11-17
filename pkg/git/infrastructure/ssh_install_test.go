package infrastructure_test

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/client"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitops "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
)

func TestNewSSHAgentAuthFromSocket(t *testing.T) {
	t.Parallel()

	t.Run("should return nil when nothing listens on the socket", func(t *testing.T) {
		t.Parallel()

		// given
		socketPath := filepath.Join(t.TempDir(), "absent.sock")

		// when
		method := gitops.NewSSHAgentAuthFromSocket(socketPath)

		// then
		assert.Nil(t, method)
	})

	t.Run("should return an agent auth method when an agent answers", func(t *testing.T) {
		t.Parallel()

		// given
		socketPath := serveSSHAgent(t, newEd25519Key(t))

		// when
		method := gitops.NewSSHAgentAuthFromSocket(socketPath)

		// then
		agentAuth, isAgentAuth := method.(*gitssh.PublicKeysCallback)
		require.True(t, isAgentAuth)
		assert.Equal(t, "git", agentAuth.User)
	})
}

// TestDetectSSHAgentSockets is deliberately not parallel: it calls t.Setenv, which the
// runtime forbids in a parallel test.
func TestDetectSSHAgentSockets(t *testing.T) {
	t.Run("should report SSH_AUTH_SOCK first when it is a socket", func(t *testing.T) {
		// given
		socketPath := filepath.Join(t.TempDir(), "agent.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		t.Setenv("SSH_AUTH_SOCK", socketPath)
		t.Setenv("HOME", t.TempDir())

		// when
		sockets := gitops.DetectSSHAgentSockets()

		// then
		assert.Equal(t, []string{socketPath}, sockets)
	})

	t.Run("should skip an SSH_AUTH_SOCK that is missing or not a socket", func(t *testing.T) {
		// given
		notASocket := writeSSHFixture(t, filepath.Join(t.TempDir(), "agent.sock"), "")
		t.Setenv("HOME", t.TempDir())

		for _, candidate := range []string{filepath.Join(t.TempDir(), "absent.sock"), notASocket} {
			t.Setenv("SSH_AUTH_SOCK", candidate)

			// when
			sockets := gitops.DetectSSHAgentSockets()

			// then
			assert.Empty(t, sockets, candidate)
		}
	})

	t.Run("should find 1Password's agent socket in the home directory", func(t *testing.T) {
		// given
		home := t.TempDir()
		socketPath := filepath.Join(makeDir(t, filepath.Join(home, ".1password")), "agent.sock")
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		t.Setenv("SSH_AUTH_SOCK", "")
		t.Setenv("HOME", home)

		// when
		sockets := gitops.DetectSSHAgentSockets()

		// then
		assert.Equal(t, []string{socketPath}, sockets)
	})
}

// TestInstallSSHTransport is deliberately not parallel: it replaces go-git's process-wide
// SSH transport, ssh_config reader and gitforge's default host key callback, and puts them
// back when it ends.
//
//nolint:paralleltest // test mutates process-wide go-git state
func TestInstallSSHTransport(t *testing.T) {
	//nolint:paralleltest // test mutates process-wide go-git state
	t.Run(
		"should install the transport, switch off go-git's ssh_config and set the fallback callback",
		func(t *testing.T) {
			// given
			previousTransport := client.Protocols["ssh"]
			previousConfig := gitssh.DefaultSSHConfig
			previousCallback := gitops.DefaultSSHHostKeyCallback()
			t.Cleanup(func() {
				client.InstallProtocol("ssh", previousTransport)
				gitssh.DefaultSSHConfig = previousConfig //nolint:reassign // restores the global the test changed
				gitops.SetDefaultSSHHostKeyCallback(previousCallback)
			})

			// when
			gitops.InstallSSHTransport()

			// then
			assert.NotEqual(t, gitssh.DefaultClient, client.Protocols["ssh"])
			assert.Nil(t, gitssh.DefaultSSHConfig)
			assert.NotNil(t, gitops.DefaultSSHHostKeyCallback())
		},
	)
}
