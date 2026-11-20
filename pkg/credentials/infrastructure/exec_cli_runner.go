package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	configHelpers "github.com/rios0rios0/gitforge/v4/pkg/config/domain/helpers"
)

// cliWaitDelay bounds how long Output waits for a CLI's output to close once the CLI has
// exited or its context has ended. A launcher can leave a child holding standard output,
// which would otherwise keep Output waiting indefinitely.
const cliWaitDelay = 2 * time.Second

// cliStderrLimit bounds how much of a CLI's standard error Output keeps for its error, as
// [exec.Cmd.Output] bounds what it keeps for an ExitError.
const cliStderrLimit = 32 << 10

// enterpriseTokenVariables are the variables gh reads a GitHub Enterprise Server token from,
// "GH_ENTERPRISE_TOKEN, GITHUB_ENTERPRISE_TOKEN (in order of precedence)". gitforge reads
// neither, so configHelpers.TokenEnvVars does not list them.
//
//nolint:gochecknoglobals // read-only lookup table
var enterpriseTokenVariables = []string{"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}

// ExecCLIRunner finds forge CLIs on PATH and runs them as child processes, with the
// environment of this process minus the variables that hold a forge token (see
// cliEnvironment), so a CLI reports only the login it stored itself.
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
// the token. The CLI gets this process's environment without the forge token variables.
//
// On Unix the CLI runs in a process group of its own, so that the context's end can kill
// everything it started, and so a terminal's interrupt no longer reaches it. While it runs,
// Output therefore relays SIGINT, SIGTERM and SIGHUP from this process to the CLI's group;
// once the CLI has exited it stops relaying and, if one of them arrived, sends the first to
// this process again, which then ends, or runs its own handler, as it would have without the
// separate group. A program with its own handler for that signal receives it twice: once
// while the CLI runs and once more after it exits.
func (r *ExecCLIRunner) Output(ctx context.Context, path string, args ...string) (string, error) {
	// #nosec G204 -- path is LookPath's absolute result for a fixed program name, and args
	// are constants plus a tenant ID validated as a GUID.
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = r.workingDirectory()
	cmd.Env = cliEnvironment(os.Environ())
	// A nil Stdin reads from the null device, so a CLI that unexpectedly prompts fails on
	// end of file instead of waiting for an answer that never comes.
	cmd.Stdin = nil
	var stdout bytes.Buffer
	stderr := &cappedBuffer{limit: cliStderrLimit}
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = cliWaitDelay
	killProcessGroupOnCancel(cmd)

	err := runCLI(cmd)
	// ErrWaitDelay after a successful exit, with the context still live, means only that
	// something the CLI started kept standard output open: the CLI itself finished and
	// printed what it had to.
	if errors.Is(err, exec.ErrWaitDelay) && ctx.Err() == nil {
		err = nil
	}
	if err == nil {
		return strings.TrimSpace(stdout.String()), nil
	}

	command := filepath.Base(path) + " " + strings.Join(args, " ")
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("%s: %w", command, ctxErr)
	}
	if exitErr, exited := errors.AsType[*exec.ExitError](err); exited {
		// exec.Cmd.Output put standard error on the exit error when it collected it; Output
		// collects it into a buffer of its own, so it fills the field the same way.
		exitErr.Stderr = []byte(stderr.String())
		if detail := strings.Join(strings.Fields(stderr.String()), " "); detail != "" {
			return "", fmt.Errorf("%s: %w: %s", command, err, detail)
		}
	}
	return "", fmt.Errorf("%s: %w", command, err)
}

// cappedBuffer keeps the first limit bytes written to it and discards the rest, so a CLI that
// floods standard error cannot grow the error Output builds from it without bound.
type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

// Write keeps what fits under the limit and reports every byte as written, so the CLI's
// output is drained whatever its size.
func (b *cappedBuffer) Write(data []byte) (int, error) {
	if room := b.limit - b.buffer.Len(); room > 0 {
		b.buffer.Write(data[:min(len(data), room)])
	}
	return len(data), nil
}

// String returns what was kept.
func (b *cappedBuffer) String() string {
	return b.buffer.String()
}

// cliEnvironment returns environ without the variables that hold a forge token: every one
// configHelpers.TokenEnvVars lists for a forge, and gh's GitHub Enterprise Server tokens. gh
// prints GH_TOKEN or GITHUB_TOKEN back when either is set (`gh auth token` reports the token
// gh would use, and they take precedence over its stored login), so the credential chain's CLI
// step would hand out the very token its environment step declined, such as one a GitHub
// Enterprise Server runner exported. Everything else is kept: a CLI needs HOME, PATH, its
// configuration directory and the proxy settings. Names are compared without regard to case,
// as Windows compares them.
func cliEnvironment(environ []string) []string {
	withheld := make(map[string]bool)
	for _, name := range enterpriseTokenVariables {
		withheld[strings.ToUpper(name)] = true
	}
	// providerCLIs has an entry for every forge: the exhaustive linter fails a literal of it
	// that leaves one out.
	for serviceType := range providerCLIs {
		for _, name := range configHelpers.TokenEnvVars(serviceType) {
			withheld[strings.ToUpper(name)] = true
		}
	}

	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !withheld[strings.ToUpper(name)] {
			kept = append(kept, entry)
		}
	}
	return kept
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
