package doubles

import "context"

// CommitSignerStub implements CommitSigner for testing.
type CommitSignerStub struct {
	SignatureValue string
	SignError      error
}

func (s *CommitSignerStub) Sign(_ context.Context, _ []byte) (string, error) {
	return s.SignatureValue, s.SignError
}
