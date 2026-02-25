package infrastructure

import (
	"context"
	"fmt"

	"github.com/rios0rios0/gitforge/pkg/signing/infrastructure/helpers"
)

// SSHSigner signs commits using an SSH key.
type SSHSigner struct {
	keyPath string
}

// NewSSHSigner creates a new SSHSigner with the given SSH key file path.
func NewSSHSigner(keyPath string) *SSHSigner {
	return &SSHSigner{keyPath: keyPath}
}

// Sign signs the commit content using ssh-keygen and returns the SSH signature.
func (s *SSHSigner) Sign(ctx context.Context, commitContent []byte) (string, error) {
	sig, err := helpers.SignSSHCommit(ctx, commitContent, s.keyPath)
	if err != nil {
		return "", fmt.Errorf("SSH signing failed: %w", err)
	}
	return sig, nil
}
