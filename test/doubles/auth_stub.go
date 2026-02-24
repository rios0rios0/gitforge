package doubles

// AuthStub is a dummy transport.AuthMethod for testing.
type AuthStub struct{}

func (s *AuthStub) Name() string   { return "stub-auth" }
func (s *AuthStub) String() string { return "stub-auth" }
