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

func TestNewSSHTransport(t *testing.T) {
	t.Parallel()

	t.Run("should build the transport with no consumer options when it is built directly", func(t *testing.T) {
		t.Parallel()

		// given -- building it reads no file: ssh_config and known_hosts wait for a connection
		next := &recordingTransport{}

		// when
		sshTransport := gitops.NewSSHTransport(next)

		// then
		options, isGitforge := gitops.SSHTransportOptionsOf(sshTransport)
		assert.True(t, isGitforge)
		assert.Equal(t, gitops.SSHTransportOptions{}, options)
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
			restoreSSHTransportGlobals(t)

			// when
			gitops.InstallSSHTransport()

			// then -- with no options, so its warnings read as they always have
			options, isGitforge := gitops.SSHTransportOptionsOf(client.Protocols["ssh"])
			assert.True(t, isGitforge)
			assert.Equal(t, gitops.SSHTransportOptions{}, options)
			assert.Nil(t, gitssh.DefaultSSHConfig)
			assert.NotNil(t, gitops.DefaultSSHHostKeyCallback())
		},
	)

	//nolint:paralleltest // test mutates process-wide go-git state
	t.Run("should install the transport with the consumer's options when it adds some", func(t *testing.T) {
		// given
		restoreSSHTransportGlobals(t)
		options := gitops.SSHTransportOptions{MissingKeyRemedy: "configure it as ssh_key_path"}

		// when
		gitops.InstallSSHTransportWith(options)

		// then
		installed, isGitforge := gitops.SSHTransportOptionsOf(client.Protocols["ssh"])
		assert.True(t, isGitforge)
		assert.Equal(t, options, installed)
		assert.Nil(t, gitssh.DefaultSSHConfig)
		assert.NotNil(t, gitops.DefaultSSHHostKeyCallback())
	})
}

// restoreSSHTransportGlobals puts go-git's SSH transport and ssh_config reader and gitforge's
// default host key callback back, as they are now, when the test ends.
func restoreSSHTransportGlobals(t *testing.T) {
	t.Helper()

	previousTransport := client.Protocols["ssh"]
	previousConfig := gitssh.DefaultSSHConfig
	previousCallback := gitops.DefaultSSHHostKeyCallback()
	t.Cleanup(func() {
		client.InstallProtocol("ssh", previousTransport)
		gitssh.DefaultSSHConfig = previousConfig //nolint:reassign // restores the global the test changed
		gitops.SetDefaultSSHHostKeyCallback(previousCallback)
	})
}
