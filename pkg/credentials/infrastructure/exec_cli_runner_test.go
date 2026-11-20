package infrastructure_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/infrastructure"
)

// posixShell is the interpreter the fixture CLIs run under.
const posixShell = "/bin/sh"

// writeScript writes a POSIX shell script into a temporary directory and returns its path.
// The fixture CLI is posixShell running this script, so the script itself needs no
// execute permission.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture CLIs are POSIX shell scripts")
	}
	path := filepath.Join(t.TempDir(), "fixture-cli.sh")
	require.NoError(t, os.WriteFile(path, []byte(body+"\n"), 0o600))
	return path
}

// TestExecCLIRunnerLookPath is deliberately not parallel: it points PATH at a temporary
// directory with t.Setenv, which the runtime forbids in a parallel test.
func TestExecCLIRunnerLookPath(t *testing.T) {
	t.Run("should return the absolute path when the program is on PATH", func(t *testing.T) {
		// given
		if runtime.GOOS == "windows" {
			t.Skip("the fixture CLI is a link to a POSIX shell")
		}
		dir := t.TempDir()
		expected := filepath.Join(dir, "fixture-cli")
		require.NoError(t, os.Symlink(posixShell, expected))
		t.Setenv("PATH", dir)
		runner := infrastructure.NewExecCLIRunner()

		// when
		path, err := runner.LookPath("fixture-cli")

		// then
		require.NoError(t, err)
		assert.Equal(t, expected, path)
	})

	t.Run("should report exec.ErrNotFound when the program is not on PATH", func(t *testing.T) {
		// given
		t.Setenv("PATH", t.TempDir())
		runner := infrastructure.NewExecCLIRunner()

		// when
		_, err := runner.LookPath("fixture-missing-cli")

		// then
		require.ErrorIs(t, err, exec.ErrNotFound)
	})

	t.Run("should refuse a program PATH lookup finds in the current directory", func(t *testing.T) {
		// given
		if runtime.GOOS == "windows" {
			t.Skip("the fixture CLI is a link to a POSIX shell")
		}
		dir := t.TempDir()
		require.NoError(t, os.Symlink(posixShell, filepath.Join(dir, "fixture-cli")))
		t.Chdir(dir)
		t.Setenv("PATH", ".")
		runner := infrastructure.NewExecCLIRunner()

		// when
		_, err := runner.LookPath("fixture-cli")

		// then
		require.ErrorIs(t, err, exec.ErrDot)
	})

	t.Run("should refuse a relative path when GODEBUG lets PATH lookup return one", func(t *testing.T) {
		// given
		if runtime.GOOS == "windows" {
			t.Skip("the fixture CLI is a link to a POSIX shell")
		}
		dir := t.TempDir()
		require.NoError(t, os.Symlink(posixShell, filepath.Join(dir, "fixture-cli")))
		t.Chdir(dir)
		t.Setenv("PATH", ".")
		t.Setenv("GODEBUG", "execerrdot=0")
		runner := infrastructure.NewExecCLIRunner()

		// when
		_, err := runner.LookPath("fixture-cli")

		// then
		require.ErrorIs(t, err, infrastructure.ErrRelativeExecutable)
	})
}

// TestExecCLIRunnerWorkingDirectory is deliberately not parallel: it changes HOME with
// t.Setenv, which the runtime forbids in a parallel test.
func TestExecCLIRunnerWorkingDirectory(t *testing.T) {
	t.Run("should run the program in the home directory when Dir is empty", func(t *testing.T) {
		// given
		home := t.TempDir()
		t.Setenv("HOME", home)
		script := writeScript(t, "pwd -P")
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		expected, evalErr := filepath.EvalSymlinks(home)
		require.NoError(t, evalErr)
		assert.Equal(t, expected, output)
	})

	t.Run("should run the program at the filesystem root when HOME is unset", func(t *testing.T) {
		// given
		t.Setenv("HOME", "")
		script := writeScript(t, "pwd -P")
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		assert.Equal(t, "/", output)
	})

	t.Run("should run the program at the filesystem root when HOME does not exist", func(t *testing.T) {
		// given
		t.Setenv("HOME", filepath.Join(t.TempDir(), "missing"))
		script := writeScript(t, "pwd -P")
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		assert.Equal(t, "/", output)
	})
}

// forgeTokenVariables are the variables that hold a forge token: gitforge's own, and gh's
// GitHub Enterprise Server tokens.
func forgeTokenVariables() []string {
	return []string{
		"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
		"GITLAB_TOKEN", "GL_TOKEN", "AZURE_DEVOPS_EXT_PAT", "SYSTEM_ACCESSTOKEN", "CODEBERG_TOKEN",
	}
}

// writeVariableReport writes a fixture CLI that prints the name of each of names that reached
// it, never a value.
func writeVariableReport(t *testing.T, names []string) string {
	t.Helper()
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, `if [ -n "${`+name+`+set}" ]; then printf '%s ' `+name+`; fi`)
	}
	return writeScript(t, strings.Join(lines, "\n"))
}

// TestExecCLIRunnerEnvironment is deliberately not parallel: it sets the variables the
// program may or may not see with t.Setenv, which the runtime forbids in a parallel test.
func TestExecCLIRunnerEnvironment(t *testing.T) {
	t.Run("should keep every forge token variable from the program when they are set", func(t *testing.T) {
		// given
		for _, name := range forgeTokenVariables() {
			t.Setenv(name, "fixture-token-placeholder")
		}
		script := writeVariableReport(t, forgeTokenVariables())
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		assert.Empty(t, output, "these forge token variables reached the program")
	})

	t.Run("should hand the program the rest of the environment when token variables are withheld", func(t *testing.T) {
		// given
		t.Setenv("GH_TOKEN", "fixture-token-placeholder")
		t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")
		t.Setenv("GITFORGE_FIXTURE_SETTING", "fixture-setting-placeholder")
		script := writeVariableReport(t, []string{"HOME", "PATH", "HTTPS_PROXY", "GITFORGE_FIXTURE_SETTING"})
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		assert.Equal(t, "HOME PATH HTTPS_PROXY GITFORGE_FIXTURE_SETTING", output)
	})
}

func TestExecCLIRunnerOutput(t *testing.T) {
	t.Parallel()

	t.Run("should return trimmed standard output when the program succeeds", func(t *testing.T) {
		t.Parallel()

		// given
		script := writeScript(t, `printf '  fixture-token\n'`)
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script, "auth", "token")

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-token", output)
	})

	t.Run("should report standard error but never standard output when the program fails", func(t *testing.T) {
		t.Parallel()

		// given
		script := writeScript(t,
			`printf 'fixture-printed-token'; printf 'not signed in\nrun the login command\n' >&2; exit 1`)
		runner := infrastructure.NewExecCLIRunner()

		// when
		_, err := runner.Output(t.Context(), posixShell, script, "auth", "token")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth token")
		assert.Contains(t, err.Error(), "not signed in run the login command")
		assert.NotContains(t, err.Error(), "fixture-printed-token")
		exitErr, exited := errors.AsType[*exec.ExitError](err)
		require.True(t, exited)
		assert.Equal(t, "not signed in\nrun the login command\n", string(exitErr.Stderr))
	})

	t.Run("should give the program no input when it tries to read some", func(t *testing.T) {
		t.Parallel()

		// given
		script := writeScript(t, `read -r line; printf 'read:%s' "$line"`)
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		assert.Equal(t, "read:", output)
	})

	t.Run("should run the program in the given directory when Dir is set", func(t *testing.T) {
		t.Parallel()

		// given
		dir := t.TempDir()
		script := writeScript(t, "pwd -P")
		runner := &infrastructure.ExecCLIRunner{Dir: dir}

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		expected, evalErr := filepath.EvalSymlinks(dir)
		require.NoError(t, evalErr)
		assert.Equal(t, expected, output)
	})

	t.Run("should stop the program and what it started when its context ends", func(t *testing.T) {
		t.Parallel()

		// given
		script := writeScript(t, "sleep 30; printf 'fixture-late-output'")
		runner := infrastructure.NewExecCLIRunner()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		started := time.Now()

		// when
		_, err := runner.Output(ctx, posixShell, script)

		// then
		require.ErrorIs(t, err, context.DeadlineExceeded)
		// The shell runs sleep as its child: only killing the whole process group returns
		// before the two seconds Output would otherwise wait for sleep's open output.
		assert.Less(t, time.Since(started), 2*time.Second)
	})

	t.Run("should return the output when the program leaves a child holding it open", func(t *testing.T) {
		t.Parallel()

		// given
		script := writeScript(t, "printf 'fixture-token\\n'; (sleep 5) & exit 0")
		runner := infrastructure.NewExecCLIRunner()

		// when
		output, err := runner.Output(t.Context(), posixShell, script)

		// then
		require.NoError(t, err)
		assert.Equal(t, "fixture-token", output)
	})
}
