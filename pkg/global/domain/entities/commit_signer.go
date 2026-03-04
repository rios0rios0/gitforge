package entities

import "context"

// CommitSigner abstracts commit signing behavior (GPG, SSH, etc.).
type CommitSigner interface {
	Sign(ctx context.Context, commitContent []byte) (string, error)
}
