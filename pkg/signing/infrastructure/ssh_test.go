package infrastructure_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infrastructure "github.com/rios0rios0/gitforge/pkg/signing/infrastructure"
)

func TestReadSSHSigningKey(t *testing.T) {
	t.Parallel()

	t.Run("should return error when signing key is empty", func(t *testing.T) {
		t.Parallel()

		// given
		keyPath := ""

		// when
		_, err := infrastructure.ReadSSHSigningKey(keyPath)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("should return error when key file does not exist", func(t *testing.T) {
		t.Parallel()

		// given
		keyPath := "/tmp/nonexistent-ssh-key-xyz-12345"

		// when
		_, err := infrastructure.ReadSSHSigningKey(keyPath)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("should return absolute path when file exists", func(t *testing.T) {
		t.Parallel()

		// given
		tmpDir := t.TempDir()
		keyFile := filepath.Join(tmpDir, "test_key")
		err := os.WriteFile(keyFile, []byte("fake-key"), 0o600)
		require.NoError(t, err)

		// when
		result, err := infrastructure.ReadSSHSigningKey(keyFile)

		// then
		require.NoError(t, err)
		assert.Equal(t, keyFile, result)
	})

	t.Run("should expand tilde and return path when file exists", func(t *testing.T) {
		t.Parallel()

		// given
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		// use a unique filename directly under home to avoid needing MkdirAll
		keyFile := filepath.Join(home, ".gitforge_test_tilde_expand_key")
		err = os.WriteFile(keyFile, []byte("fake-key"), 0o600)
		require.NoError(t, err)
		defer os.Remove(keyFile)

		// when
		result, err := infrastructure.ReadSSHSigningKey("~/.gitforge_test_tilde_expand_key")

		// then
		require.NoError(t, err)
		assert.Equal(t, keyFile, result)
	})
}

func TestSSHSignerSign(t *testing.T) {
	t.Parallel()

	t.Run("should return error when ssh-keygen is given invalid key", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		tmpDir := t.TempDir()
		fakeKey := filepath.Join(tmpDir, "bad_key")
		err = os.WriteFile(fakeKey, []byte("not-a-real-key"), 0o600)
		require.NoError(t, err)

		signer := infrastructure.NewSSHSigner(fakeKey)
		content := []byte("tree abc\nauthor test\n\ncommit message")

		// when
		_, err = signer.Sign(context.Background(), content)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SSH signing failed")
	})

	t.Run("should sign commit content with valid SSH key", func(t *testing.T) {
		t.Parallel()

		// given
		_, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Skip("ssh-keygen not available")
		}

		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "test_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
		err = cmd.Run()
		require.NoError(t, err)

		signer := infrastructure.NewSSHSigner(keyPath)
		content := []byte("tree 0000000000000000000000000000000000000000\nauthor Test <test@test.com>\n\ntest commit")

		// when
		sig, err := signer.Sign(context.Background(), content)

		// then
		require.NoError(t, err)
		assert.Contains(t, sig, "-----BEGIN SSH SIGNATURE-----")
		assert.Contains(t, sig, "-----END SSH SIGNATURE-----")
	})
}
