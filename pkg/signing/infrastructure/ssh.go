package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	defer func() {
		if cerr := tmpFile.Close(); cerr != nil {
			log.WithError(cerr).Warn("failed to close temp file for SSH signing")
		}
		if rerr := os.Remove(tmpFile.Name()); rerr != nil { //nolint:gosec // tmpFile.Name() is not user-controlled
			log.WithError(rerr).Warn("failed to remove temp file for SSH signing")
		}
	}()

	if _, err = tmpFile.Write(commitContent); err != nil {
		return "", fmt.Errorf("failed to write commit content for SSH signing: %w", err)
	}

	sigFile := tmpFile.Name() + ".sig"
	defer os.Remove(sigFile)

	cmd := exec.CommandContext( //nolint:gosec // signingKeyPath is validated by ReadSSHSigningKey
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

	sigBytes, err := os.ReadFile(sigFile) //nolint:gosec // derived from tmpFile.Name(), not user-controlled
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
		return "", errors.New("no SSH signing key configured (user.signingkey is empty)")
	}

	if strings.HasPrefix(signingKey, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		signingKey = filepath.Join(home, signingKey[2:])
	} else if signingKey == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		signingKey = home
	}

	if _, err := os.Stat(signingKey); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("SSH signing key file not found: %s", signingKey)
		}
		return "", fmt.Errorf("failed to stat SSH signing key file %s: %w", signingKey, err)
	}

	return signingKey, nil
}
