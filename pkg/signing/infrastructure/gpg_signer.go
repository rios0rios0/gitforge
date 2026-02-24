package infrastructure

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// GPGSigner signs commits using a GPG key.
type GPGSigner struct {
	key *openpgp.Entity
}

// NewGPGSigner creates a new GPGSigner with the given GPG key entity.
func NewGPGSigner(key *openpgp.Entity) *GPGSigner {
	return &GPGSigner{key: key}
}

// Key returns the underlying GPG key entity for use with go-git's SignKey option.
func (s *GPGSigner) Key() *openpgp.Entity {
	return s.key
}

// Sign signs the commit content using the GPG key and returns the armored signature.
func (s *GPGSigner) Sign(_ context.Context, commitContent []byte) (string, error) {
	var buf bytes.Buffer
	err := openpgp.ArmoredDetachSign(&buf, s.key, bytes.NewReader(commitContent), nil)
	if err != nil {
		return "", fmt.Errorf("GPG signing failed: %w", err)
	}
	return buf.String(), nil
}
