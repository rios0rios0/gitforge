package infrastructure_test

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	gitops "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
)

func TestSSHAgentSigners(t *testing.T) {
	t.Parallel()

	t.Run("should list the agent's keys in the agent's order", func(t *testing.T) {
		t.Parallel()

		// given
		first, second := newEd25519Key(t), newEd25519Key(t)
		listSigners := agentSignerCallback(t, serveSSHAgent(t, first, second))

		// when
		signers, err := listSigners()

		// then
		require.NoError(t, err)
		require.Len(t, signers, 2)
		assert.Equal(t, newTestSigner(t, first).PublicKey().Marshal(), signers[0].PublicKey().Marshal())
		assert.Equal(t, newTestSigner(t, second).PublicKey().Marshal(), signers[1].PublicKey().Marshal())
	})

	t.Run("should still sign after the callback that listed the keys returned", func(t *testing.T) {
		t.Parallel()

		// given -- x/crypto signs only after the callback returns; a signer bound to the
		// connection that listed the keys used to fail here with "use of closed network
		// connection", which failed every agent-based push gitforge attempted
		listSigners := agentSignerCallback(t, serveSSHAgent(t, newEd25519Key(t)))
		signers, err := listSigners()
		require.NoError(t, err)
		require.Len(t, signers, 1)
		data := []byte("session identifier the server asks to be signed")

		// when
		signature, err := signers[0].Sign(rand.Reader, data)

		// then
		require.NoError(t, err)
		require.NoError(t, signers[0].PublicKey().Verify(data, signature))
	})

	t.Run("should sign with rsa-sha2-256 when the handshake asks for it", func(t *testing.T) {
		t.Parallel()

		// given -- most servers now refuse the SHA-1 ssh-rsa signature, so an RSA key has to
		// be able to sign with a SHA-2 algorithm through the agent
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		listSigners := agentSignerCallback(t, serveSSHAgent(t, rsaKey))
		signers, err := listSigners()
		require.NoError(t, err)
		require.Len(t, signers, 1)
		algorithmSigner, isAlgorithmSigner := signers[0].(ssh.AlgorithmSigner)
		require.True(t, isAlgorithmSigner, "x/crypto negotiates SHA-2 only for an ssh.AlgorithmSigner")
		data := []byte("session identifier the server asks to be signed")

		// when
		signature, err := algorithmSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA256)

		// then
		require.NoError(t, err)
		assert.Equal(t, ssh.KeyAlgoRSASHA256, signature.Format)
		require.NoError(t, signers[0].PublicKey().Verify(data, signature))
	})

	t.Run("should refuse an algorithm the agent protocol cannot ask for", func(t *testing.T) {
		t.Parallel()

		// given -- DSA has no agent signature flag, and is not the Ed25519 key's own algorithm
		const unsupportedAlgorithm = "ssh-dss"
		listSigners := agentSignerCallback(t, serveSSHAgent(t, newEd25519Key(t)))
		signers, err := listSigners()
		require.NoError(t, err)
		require.Len(t, signers, 1)
		algorithmSigner, isAlgorithmSigner := signers[0].(ssh.AlgorithmSigner)
		require.True(t, isAlgorithmSigner)

		// when
		_, err = algorithmSigner.SignWithAlgorithm(rand.Reader, []byte("data"), unsupportedAlgorithm)

		// then
		require.ErrorContains(t, err, unsupportedAlgorithm)
	})
}

func TestSSHAgentListingRetries(t *testing.T) {
	t.Parallel()

	t.Run("should list the keys again when the relay dropped the connection", func(t *testing.T) {
		t.Parallel()

		// given -- connection 1 is the liveness probe; the relay then drops connection 2
		keyring := agent.NewKeyring()
		require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: newEd25519Key(t)}))
		scripted := serveScriptedSSHAgent(t, func(number int, conn net.Conn) {
			if number == 2 {
				return
			}
			_ = agent.ServeAgent(keyring, conn)
		})
		listSigners := agentSignerCallback(t, scripted.socketPath)

		// when
		signers, err := listSigners()

		// then
		require.NoError(t, err)
		assert.Len(t, signers, 1)
		assert.Equal(t, 3, scripted.connectionCount(), "the probe, the dropped listing and the retry")
	})

	t.Run("should not list again when the agent refused", func(t *testing.T) {
		t.Parallel()

		// given -- a refusal is the agent's answer, and asking again would ask the operator again
		scripted := serveScriptedSSHAgent(t, func(_ int, conn net.Conn) {
			_ = agent.ServeAgent(refusingSSHAgent{Agent: agent.NewKeyring()}, conn)
		})
		listSigners := agentSignerCallback(t, scripted.socketPath)

		// when
		_, err := listSigners()

		// then
		require.Error(t, err)
		assert.Equal(t, 2, scripted.connectionCount(), "the probe and one listing")
	})

	t.Run("should give up when the relay keeps dropping the connection", func(t *testing.T) {
		t.Parallel()

		// given
		scripted := serveScriptedSSHAgent(t, func(int, net.Conn) {})
		listSigners := agentSignerCallback(t, scripted.socketPath)

		// when
		_, err := listSigners()

		// then
		require.ErrorContains(t, err, "cannot list the SSH agent's keys")
		assert.Equal(t, 4, scripted.connectionCount(), "the probe and three listings")
	})

	t.Run("should list again when the relay dropped the connection before the request was written", func(t *testing.T) {
		t.Parallel()

		// given -- every connection is closed on the agent's side before the client writes
		var dials atomic.Int32
		dial := func() (net.Conn, error) {
			dials.Add(1)
			client, agentSide := net.Pipe()
			_ = agentSide.Close()
			return client, nil
		}

		// when
		_, err := gitops.ListSSHAgentSigners(dial)

		// then -- a request that never reached the agent cannot have asked anyone anything
		require.Error(t, err)
		assert.Equal(t, int32(3), dials.Load(), "three listings")
	})

	t.Run("should list again when the relay dropped the connection before the answer", func(t *testing.T) {
		t.Parallel()

		// given -- the agent's side reads the request and closes without answering
		var dials atomic.Int32
		dial := func() (net.Conn, error) {
			dials.Add(1)
			client, agentSide := net.Pipe()
			go func() {
				_, _ = agentSide.Read(make([]byte, 64))
				_ = agentSide.Close()
			}()
			return client, nil
		}

		// when
		_, err := gitops.ListSSHAgentSigners(dial)

		// then
		require.Error(t, err)
		assert.Equal(t, int32(3), dials.Load(), "three listings")
	})

	t.Run("should not list again when the agent refused and then closed the connection", func(t *testing.T) {
		t.Parallel()

		// given -- the close comes back with the refusal's last byte, so a read has failed
		// before the refusal reaches the caller, as when x/crypto's reader goroutine reads on
		// past an answer and finds the agent gone
		var dials atomic.Int32
		dial := func() (net.Conn, error) {
			dials.Add(1)
			return newRefusingThenClosingSSHAgentConn(), nil
		}

		// when
		_, err := gitops.ListSSHAgentSigners(dial)

		// then -- x/crypto read the whole refusal, which is the agent's answer however the
		// connection ended after it
		require.ErrorContains(t, err, "agent: failed to list keys")
		assert.Equal(t, int32(1), dials.Load(), "one listing")
	})
}

// agentSignerCallback returns the signer callback of the agent auth method gitforge builds
// for socketPath.
func agentSignerCallback(t *testing.T, socketPath string) func() ([]ssh.Signer, error) {
	t.Helper()

	method := gitops.NewSSHAgentAuthFromSocket(socketPath)
	agentAuth, isAgentAuth := method.(*gitssh.PublicKeysCallback)
	require.True(t, isAgentAuth, "the agent auth method must be a PublicKeysCallback")
	return agentAuth.Callback
}

// refusingThenClosingSSHAgentConn is the client's end of a connection to an agent that
// answers the request with SSH_AGENT_FAILURE and closes the connection, the close arriving
// with the answer's last byte: a read may return data and [io.EOF] together, and
// [io.ReadFull] still counts the answer whole. So a read has failed before the refusal
// reaches the caller every time; over a socket, that depends on x/crypto's reader goroutine
// reading on past the answer before the caller looks.
type refusingThenClosingSSHAgentConn struct {
	net.Conn // left nil: the agent client only reads, writes and closes

	answer      []byte
	requested   chan struct{}
	closed      chan struct{}
	requestOnce sync.Once
	closeOnce   sync.Once
}

// newRefusingThenClosingSSHAgentConn returns a connection whose agent has not been asked
// anything yet.
func newRefusingThenClosingSSHAgentConn() *refusingThenClosingSSHAgentConn {
	return &refusingThenClosingSSHAgentConn{
		// a message length of 1, big-endian, then the message: SSH_AGENT_FAILURE
		answer:    []byte{0x00, 0x00, 0x00, 0x01, 0x05},
		requested: make(chan struct{}),
		closed:    make(chan struct{}),
	}
}

// Read serves the refusal once the request was written, returning [io.EOF] with its last
// byte, and nothing but [io.EOF] after it.
func (c *refusingThenClosingSSHAgentConn) Read(buffer []byte) (int, error) {
	select {
	case <-c.requested:
	case <-c.closed:
		return 0, net.ErrClosed
	}

	read := copy(buffer, c.answer)
	c.answer = c.answer[read:]
	if len(c.answer) == 0 {
		return read, io.EOF
	}
	return read, nil
}

// Write accepts the request.
func (c *refusingThenClosingSSHAgentConn) Write(request []byte) (int, error) {
	c.requestOnce.Do(func() { close(c.requested) })
	return len(request), nil
}

// Close closes the connection, releasing a read still waiting for the request.
func (c *refusingThenClosingSSHAgentConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}
