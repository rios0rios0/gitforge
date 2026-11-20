package doubles

import (
	"net/http"
	"sync"
)

// RoundTripperStub is an [http.RoundTripper] that fails every request with Err, without network
// access, the way a request to a host that cannot be reached fails. It records the URL of every
// request it was handed.
type RoundTripperStub struct {
	Err error

	mu   sync.Mutex
	urls []string
}

// RoundTrip records the request's URL and fails it with Err.
func (s *RoundTripperStub) RoundTrip(request *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.urls = append(s.urls, request.URL.String())
	s.mu.Unlock()
	return nil, s.Err
}

// URLs returns the URL of every request RoundTrip was handed.
func (s *RoundTripperStub) URLs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.urls...)
}
