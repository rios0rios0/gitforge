package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/domain/entities"
)

// ErrRelativeExecutable reports a CLI that PATH lookup resolved to a relative path, which
// would run a program from the current directory.
var ErrRelativeExecutable = errors.New("refusing to run an executable found at a relative path")

// unavailableError explains why one credential source could not supply a token. It
// matches entities.ErrNoCredential, so a caller tests any resolver's failure the same way,
// while its text stays the bare reason a chain lists beside the others.
type unavailableError struct {
	reason string
	cause  error
}

func newUnavailableError(cause error, format string, args ...any) *unavailableError {
	return &unavailableError{reason: fmt.Sprintf(format, args...), cause: cause}
}

func (e *unavailableError) Error() string {
	if e.cause == nil {
		return e.reason
	}
	return e.reason + ": " + e.cause.Error()
}

// Is reports the error as an entities.ErrNoCredential.
func (e *unavailableError) Is(target error) bool {
	return target == entities.ErrNoCredential
}

// Unwrap exposes the underlying cause, such as a context deadline.
func (e *unavailableError) Unwrap() error {
	return e.cause
}

// noCredentialError is what a chain returns when every source declined: one line naming
// the forge and each source's reason, in the order the sources were tried.
type noCredentialError struct {
	forge   string
	reasons []error
}

func (e *noCredentialError) Error() string {
	if len(e.reasons) == 0 {
		return fmt.Sprintf("no %s credential available: no credential source is configured", e.forge)
	}
	texts := make([]string, 0, len(e.reasons))
	for _, reason := range e.reasons {
		texts = append(texts, reason.Error())
	}
	return fmt.Sprintf("no %s credential available: %s", e.forge, strings.Join(texts, "; "))
}

// Is reports the error as an entities.ErrNoCredential.
func (e *noCredentialError) Is(target error) bool {
	return target == entities.ErrNoCredential
}

// Unwrap exposes each source's reason.
func (e *noCredentialError) Unwrap() []error {
	return e.reasons
}
