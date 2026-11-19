package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// cliWaitDelay bounds how long Output waits for a CLI's output to close once the CLI has
// exited or its context has ended. A launcher can leave a child holding standard output,
// which would otherwise keep Output waiting indefinitely.
const cliWaitDelay = 2 * time.Second

// ExecCLIRunner finds forge CLIs on PATH and runs them as child processes.
type ExecCLIRunner struct {
	// Dir is the directory the CLI runs in. Empty means the user's home directory, or the
	// filesystem root when there is none, and never the caller's own directory: the
	// consumers work inside a repository they were pointed at, and a CLI such as a
	// pip-installed az puts its working directory on Python's module path, so a file in
	// that checkout would run as the user while their credentials are at hand.
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
	killProcessGroupOnCancel(cmd)

	output, err := cmd.Output()
	// ErrWaitDelay after a successful exit, with the context still live, means only that
	// something the CLI started kept standard output open: the CLI itself finished and
	// printed what it had to.
	if errors.Is(err, exec.ErrWaitDelay) && ctx.Err() == nil {
		err = nil
	}
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

// workingDirectory returns Dir, else the user's home directory when it exists, else the
// filesystem root. It never returns "", which would run the CLI in the caller's directory.
func (r *ExecCLIRunner) workingDirectory() string {
	if r.Dir != "" {
		return r.Dir
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if info, statErr := os.Stat(home); statErr == nil && info.IsDir() {
			return home
		}
	}
	return filesystemRoot()
}

// filesystemRoot returns the root of the filesystem, or of the system drive on Windows.
func filesystemRoot() string {
	if runtime.GOOS != "windows" {
		return string(filepath.Separator)
	}
	if drive := os.Getenv("SystemDrive"); drive != "" {
		return drive + string(filepath.Separator)
	}
	return `C:\`
}
