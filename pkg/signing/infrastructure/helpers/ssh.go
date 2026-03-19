package helpers

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

// isInlineSSHKey returns true when the signing key value is an inline public key string
// (e.g. "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... user@host") rather than a file path.
// Detection requires at least two whitespace-separated fields (key-type + base64 data)
// and a recognized key-type prefix, preventing misclassification of file paths like
// "./ssh-ed25519".
func isInlineSSHKey(signingKey string) bool {
	fields := strings.Fields(signingKey)
	if len(fields) < 2 { //nolint:mnd // inline SSH keys always have at least key-type + data
		return false
	}
	keyType := fields[0]
	return strings.HasPrefix(keyType, "ssh-") ||
		strings.HasPrefix(keyType, "ecdsa-") ||
		strings.HasPrefix(keyType, "sk-")
}

// SignSSHCommit signs commit content using an SSH signing program and returns the signature.
// It uses the `-Y sign` interface which is the same mechanism Git uses internally.
// When signingKeyRef is an inline public key (detected via isInlineSSHKey), it writes
// the key to a temp file and passes `-U` so the program signs via the SSH agent.
// sshProgram overrides the signing binary (e.g. "op-ssh-sign-wsl"); empty defaults to "ssh-keygen".
func SignSSHCommit(
	ctx context.Context, commitContent []byte, signingKeyRef, sshProgram string,
) (string, error) {
	if sshProgram == "" {
		sshProgram = "ssh-keygen"
	}
	log.Infof("Signing commit with SSH program: %s", sshProgram)

	tmpFile, err := os.CreateTemp("", "gitforge-ssh-sign-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for SSH signing: %w", err)
	}
	defer func() {
		if cerr := tmpFile.Close(); cerr != nil {
			log.WithError(cerr).Warn("failed to close temp file for SSH signing")
		}
		if rerr := os.Remove(tmpFile.Name()); rerr != nil {
			log.WithError(rerr).Warn("failed to remove temp file for SSH signing")
		}
	}()

	if _, err = tmpFile.Write(commitContent); err != nil {
		return "", fmt.Errorf("failed to write commit content for SSH signing: %w", err)
	}

	sigFile := tmpFile.Name() + ".sig"
	defer os.Remove(sigFile)

	args, cleanup, err := buildSSHSignArgs(signingKeyRef, tmpFile.Name())
	if err != nil {
		return "", err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, sshProgram, args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"%s signing failed: %w (output: %s)", sshProgram, err, strings.TrimSpace(string(output)),
		)
	}

	sigBytes, err := os.ReadFile(sigFile)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH signature file: %w", err)
	}

	log.Info("Successfully signed commit with SSH key")
	return string(sigBytes), nil
}

// buildSSHSignArgs constructs the ssh-keygen arguments for signing.
// For file-based keys: -Y sign -f <path> -n git <file>
// For inline keys:     -Y sign -f <temp-pubkey-file> -U -n git <file>
// Returns the args slice, a cleanup function, and any error.
func buildSSHSignArgs(signingKeyRef, contentFile string) ([]string, func(), error) {
	noop := func() {}

	if !isInlineSSHKey(signingKeyRef) {
		args := []string{"-Y", "sign", "-f", signingKeyRef, "-n", "git", contentFile}
		return args, noop, nil
	}

	pubKeyFile, err := os.CreateTemp("", "gitforge-ssh-pubkey-*")
	if err != nil {
		return nil, noop, fmt.Errorf("failed to create temp file for SSH public key: %w", err)
	}

	cleanup := func() {
		if cerr := pubKeyFile.Close(); cerr != nil {
			log.WithError(cerr).Warn("failed to close temp public key file")
		}
		if rerr := os.Remove(pubKeyFile.Name()); rerr != nil {
			log.WithError(rerr).Warn("failed to remove temp public key file")
		}
	}

	if _, err = pubKeyFile.WriteString(signingKeyRef); err != nil {
		cleanup()
		return nil, noop, fmt.Errorf("failed to write inline public key to temp file: %w", err)
	}

	args := []string{"-Y", "sign", "-f", pubKeyFile.Name(), "-U", "-n", "git", contentFile}
	return args, cleanup, nil
}

// ReadSSHSigningKey resolves the SSH signing key reference from the git config value.
// It handles two modes:
//   - File path: expands ~ to the home directory and verifies the file exists (existing behavior).
//   - Inline public key (starts with "ssh-", "ecdsa-", or "sk-"): when using the default
//     ssh-keygen (sshProgram is empty), verifies SSH_AUTH_SOCK is set.  Custom signing
//     programs (e.g. op-ssh-sign-wsl) handle agent communication internally.
//
// Exported for use by autobump (github.com/rios0rios0/autobump).
func ReadSSHSigningKey(signingKey, sshProgram string) (string, error) {
	if signingKey == "" {
		return "", errors.New("no SSH signing key configured (user.signingkey is empty)")
	}

	if isInlineSSHKey(signingKey) {
		// Custom signing programs (e.g. 1Password's op-ssh-sign-wsl) handle agent
		// communication internally, so SSH_AUTH_SOCK is only required when using
		// the default ssh-keygen.
		if sshProgram == "" && os.Getenv("SSH_AUTH_SOCK") == "" {
			return "", errors.New(
				"SSH agent not available (SSH_AUTH_SOCK not set); required for inline key signing with ssh-keygen",
			)
		}
		return signingKey, nil
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
