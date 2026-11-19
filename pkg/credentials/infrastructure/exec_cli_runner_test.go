package infrastructure_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	t.Run("should run the program in the home directory when Dir is empty", func(t *testing.T) {
		t.Parallel()

		// given
		home, homeErr := os.UserHomeDir()
		require.NoError(t, homeErr)
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

	t.Run("should stop the program when its context ends", func(t *testing.T) {
		t.Parallel()

		// given
		script := writeScript(t, "exec sleep 30")
		runner := infrastructure.NewExecCLIRunner()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		started := time.Now()

		// when
		_, err := runner.Output(ctx, posixShell, script)

		// then
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(started), 10*time.Second)
	})
}
