package doubles

import (
	"context"
	"os/exec"
	"slices"
	"sync"
)

// CLIRunnerStub is a recording credentials CLIRunner: it answers LookPath and Output
// with canned values and remembers every command it was asked to run.
type CLIRunnerStub struct {
	// Paths maps a program name to the path LookPath returns. A name missing from it
	// fails with LookPathErr, or with exec.ErrNotFound when that is nil.
	Paths       map[string]string
	LookPathErr error
	// Token is what every Output call prints, unless OutputErr is set.
	Token     string
	OutputErr error
	// Block makes Output wait until its context ends, then return the context's error.
	Block bool

	mu    sync.Mutex
	calls [][]string
}

// LookPath returns the configured path for name.
func (s *CLIRunnerStub) LookPath(name string) (string, error) {
	if path, ok := s.Paths[name]; ok {
		return path, nil
	}
	if s.LookPathErr != nil {
		return "", s.LookPathErr
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// Output records the command and returns the configured token or error.
func (s *CLIRunnerStub) Output(ctx context.Context, path string, args ...string) (string, error) {
	s.mu.Lock()
	s.calls = append(s.calls, append([]string{path}, args...))
	s.mu.Unlock()

	if s.Block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if s.OutputErr != nil {
		return "", s.OutputErr
	}
	return s.Token, nil
}

// Calls returns every command Output ran, each as the path followed by its arguments.
func (s *CLIRunnerStub) Calls() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}
