package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// cliWaitDelay bounds how long a CLI's output may stay open once its context is done. A
// killed program can leave a child holding standard output, which would otherwise keep
// Output waiting after the deadline.
const cliWaitDelay = 2 * time.Second

// ExecCLIRunner finds forge CLIs on PATH and runs them as child processes.
type ExecCLIRunner struct {
	// Dir is the directory the CLI runs in. Empty means the user's home directory: the
	// consumers work inside a repository they were pointed at, and nothing in that
	// checkout should reach the program that hands over the user's credentials.
	Dir string
}

// NewExecCLIRunner creates a runner that runs CLIs from the user's home directory.
func NewExecCLIRunner() *ExecCLIRunner {
	return &ExecCLIRunner{}
}

// LookPath returns the absolute path PATH lookup finds for name.
func (r *ExecCLIRunner) LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("look for %s on PATH: %w", name, err)
	}
	// exec.LookPath returns a relative path only when GODEBUG=execerrdot=0 opts back into
	// running programs from the current directory; never honour that here.
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: %s resolved to %q", ErrRelativeExecutable, name, path)
	}
	return path, nil
}

// Output runs the executable at path and returns its trimmed standard output. On failure
// the error reports standard error only, because standard output is where a CLI prints
// the token.
func (r *ExecCLIRunner) Output(ctx context.Context, path string, args ...string) (string, error) {
	// #nosec G204 -- path is LookPath's absolute result for a fixed program name, and args
	// are constants plus a tenant ID validated as a GUID.
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = r.workingDirectory()
	// A nil Stdin reads from the null device, so a CLI that unexpectedly prompts fails on
	// end of file instead of waiting for an answer that never comes.
	cmd.Stdin = nil
	cmd.WaitDelay = cliWaitDelay

	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	command := filepath.Base(path) + " " + strings.Join(args, " ")
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("%s: %w", command, ctxErr)
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if detail := strings.Join(strings.Fields(string(exitErr.Stderr)), " "); detail != "" {
			return "", fmt.Errorf("%s: %w: %s", command, err, detail)
		}
	}
	return "", fmt.Errorf("%s: %w", command, err)
}

// workingDirectory returns Dir, else the user's home directory, else "" to inherit the
// caller's.
func (r *ExecCLIRunner) workingDirectory() string {
	if r.Dir != "" {
		return r.Dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}
