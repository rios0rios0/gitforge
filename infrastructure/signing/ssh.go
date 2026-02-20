package signing

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SignSSHCommit signs commit content using ssh-keygen and returns the SSH signature.
// It uses `ssh-keygen -Y sign` which is the same mechanism Git uses internally.
func SignSSHCommit(ctx context.Context, commitContent []byte, signingKeyPath string) (string, error) {
	log.Info("Signing commit with SSH key")

	tmpFile, err := os.CreateTemp("", "gitforge-ssh-sign-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for SSH signing: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err = tmpFile.Write(commitContent); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write commit content for SSH signing: %w", err)
	}
	tmpFile.Close()

	sigFile := tmpFile.Name() + ".sig"
	defer os.Remove(sigFile)

	cmd := exec.CommandContext(
		ctx, "ssh-keygen",
		"-Y", "sign",
		"-f", signingKeyPath,
		"-n", "git",
		tmpFile.Name(),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen signing failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	sigBytes, err := os.ReadFile(sigFile)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH signature file: %w", err)
	}

	log.Info("Successfully signed commit with SSH key")
	return string(sigBytes), nil
}

// ReadSSHSigningKey resolves the SSH signing key path from the git config value.
// It expands ~ to the home directory and verifies the file exists.
func ReadSSHSigningKey(signingKey string) (string, error) {
	if signingKey == "" {
		return "", fmt.Errorf("no SSH signing key configured (user.signingkey is empty)")
	}

	if strings.HasPrefix(signingKey, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		signingKey = home + signingKey[1:]
	}

	if _, err := os.Stat(signingKey); os.IsNotExist(err) {
		return "", fmt.Errorf("SSH signing key file not found: %s", signingKey)
	}

	return signingKey, nil
}
