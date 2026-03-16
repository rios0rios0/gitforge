package infrastructure

import (
	"context"
	"fmt"

	"github.com/rios0rios0/gitforge/pkg/signing/infrastructure/helpers"
)

// SSHSigner signs commits using an SSH key.
// The keyRef field holds either a file path to a private key or an inline public key string
// (e.g. "ssh-ed25519 AAAAC3...") for ssh-agent-based signing.
type SSHSigner struct {
	keyRef string
}

// NewSSHSigner creates a new SSHSigner with the given SSH key reference.
// keyRef may be a file path to a private key or an inline public key string for agent signing.
func NewSSHSigner(keyRef string) *SSHSigner {
	return &SSHSigner{keyRef: keyRef}
}

// Sign signs the commit content using ssh-keygen and returns the SSH signature.
func (s *SSHSigner) Sign(ctx context.Context, commitContent []byte) (string, error) {
	sig, err := helpers.SignSSHCommit(ctx, commitContent, s.keyRef)
	if err != nil {
		return "", fmt.Errorf("SSH signing failed: %w", err)
	}
	return sig, nil
}
