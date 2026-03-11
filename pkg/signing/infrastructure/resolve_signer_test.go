//go:build unit

package infrastructure_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	signingInfra "github.com/rios0rios0/gitforge/pkg/signing/infrastructure"
)

func TestResolveSignerFromGitConfig(t *testing.T) {
	t.Parallel()

	t.Run("should return nil signer when gpgSign is false", func(t *testing.T) {
		t.Parallel()

		// given
		gpgSign := "false"

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig(gpgSign, "", "", "", "", "test")

		// then
		require.NoError(t, err)
		assert.Nil(t, signer)
	})

	t.Run("should return nil signer when gpgSign is empty", func(t *testing.T) {
		t.Parallel()

		// given
		gpgSign := ""

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig(gpgSign, "", "", "", "", "test")

		// then
		require.NoError(t, err)
		assert.Nil(t, signer)
	})

	t.Run("should return nil signer when gpgSign is no", func(t *testing.T) {
		t.Parallel()

		// given
		gpgSign := "no"

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig(gpgSign, "", "", "", "", "test")

		// then
		require.NoError(t, err)
		assert.Nil(t, signer)
	})

	t.Run("should return nil signer when gpgSign is 0", func(t *testing.T) {
		t.Parallel()

		// given
		gpgSign := "0"

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig(gpgSign, "", "", "", "", "test")

		// then
		require.NoError(t, err)
		assert.Nil(t, signer)
	})

	t.Run("should return SSHSigner when format is ssh and key exists", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		keyPath := filepath.Join(t.TempDir(), "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		require.NoError(t, cmd.Run())

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig("true", "ssh", keyPath, "", "", "test")

		// then
		require.NoError(t, err)
		assert.NotNil(t, signer)
	})

	t.Run("should return error when format is ssh and key does not exist", func(t *testing.T) {
		t.Parallel()

		// given / when
		signer, err := signingInfra.ResolveSignerFromGitConfig(
			"true", "ssh", "/tmp/nonexistent-key-xyz-12345", "", "", "test",
		)

		// then
		require.Error(t, err)
		assert.Nil(t, signer)
	})

	t.Run("should return error when GPG signing and signingKey is empty", func(t *testing.T) {
		t.Parallel()

		// given / when
		signer, err := signingInfra.ResolveSignerFromGitConfig("true", "gpg", "", "", "", "test")

		// then
		require.Error(t, err)
		assert.Nil(t, signer)
		assert.Contains(t, err.Error(), "user.signingkey is required")
	})

	t.Run("should treat yes as truthy for gpgSign", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		keyPath := filepath.Join(t.TempDir(), "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		require.NoError(t, cmd.Run())

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig("yes", "ssh", keyPath, "", "", "test")

		// then
		require.NoError(t, err)
		assert.NotNil(t, signer)
	})

	t.Run("should treat on as truthy for gpgSign", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		keyPath := filepath.Join(t.TempDir(), "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		require.NoError(t, cmd.Run())

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig("on", "ssh", keyPath, "", "", "test")

		// then
		require.NoError(t, err)
		assert.NotNil(t, signer)
	})

	t.Run("should treat 1 as truthy for gpgSign", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		keyPath := filepath.Join(t.TempDir(), "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		require.NoError(t, cmd.Run())

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig("1", "ssh", keyPath, "", "", "test")

		// then
		require.NoError(t, err)
		assert.NotNil(t, signer)
	})

	t.Run("should be case-insensitive for gpgSign", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		keyPath := filepath.Join(t.TempDir(), "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		require.NoError(t, cmd.Run())

		// when
		signer, err := signingInfra.ResolveSignerFromGitConfig("TRUE", "ssh", keyPath, "", "", "test")

		// then
		require.NoError(t, err)
		assert.NotNil(t, signer)
	})
}
