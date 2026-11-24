package infrastructure_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rios0rios0/gitforge/v4/pkg/credentials/infrastructure"
)

// signalConsumerVariable selects the consumer mode of this test binary: when it names a
// directory, TestMain runs runSignalledConsumer on the stub CLI there instead of the tests.
// The signal test re-runs the binary that way, so it has a program of its own to signal.
const signalConsumerVariable = "GITFORGE_FIXTURE_SIGNAL_CONSUMER"

// The files of the signal fixture, in the directory signalConsumerVariable names.
const (
	stubCLIFile    = "stub-cli.sh"
	stubPIDFile    = "stub-cli.pid"
	stubMarkerFile = "stub-cli.signal"
)

// consumerSurvived is the exit status of a consumer that outlived the signal it was sent.
const consumerSurvived = 3

// sighup is [syscall.SIGHUP], which js/wasm does not define, so this file compiles there too:
// XSI fixes the hangup signal as number 1, the signal `kill -1` sends.
const sighup = syscall.Signal(1)

// stubCLIScript is the CLI the consumer runs. It traps INT, TERM and HUP, writing the name of
// the signal to the marker file ($1) and exiting, and otherwise sleeps. sleep runs in the
// foreground, so a trap runs only after sleep has ended: a signal that reaches the shell but
// not the rest of its process group leaves the stub asleep.
//
// Its process ID ($2) tells the test it may send the signal, so the process that sleeps writes
// it: a subshell, which starts with the trapped signals back at their default action, writes
// the shell's ID ($$ keeps it in a subshell) and then becomes sleep. Written by the shell before
// it started sleep, the ID let the signal arrive while the shell was still looking sleep up or
// starting it: only the shell got the signal, its trap waited out the 30 seconds of sleep, and
// the consumer's deadline killed the stub first.
const stubCLIScript = `trap 'printf INT > "$1"; exit 0' INT
trap 'printf TERM > "$1"; exit 0' TERM
trap 'printf HUP > "$1"; exit 0' HUP
(printf '%s' "$$" > "$2"; exec sleep 30)
`

func TestMain(m *testing.M) {
	if directory := os.Getenv(signalConsumerVariable); directory != "" {
		os.Exit(runSignalledConsumer(directory))
	}
	os.Exit(m.Run())
}

// runSignalledConsumer is a program that asks a CLI for a token and is sent a signal
// meanwhile. Output passes the signal on to the CLI and, once the CLI has exited, to this
// process again, which then dies of it: returning at all means it did not.
func runSignalledConsumer(directory string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := infrastructure.NewExecCLIRunner().Output(
		ctx, posixShell,
		filepath.Join(directory, stubCLIFile),
		filepath.Join(directory, stubMarkerFile),
		filepath.Join(directory, stubPIDFile),
	)
	// The signal Output sends this process again is delivered asynchronously: give it the time
	// to take effect before reporting that it did not.
	time.Sleep(5 * time.Second)
	fmt.Fprintf(os.Stderr, "the consumer outlived its signal; Output returned %v\n", err)
	return consumerSurvived
}

func TestExecCLIRunnerSignals(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name   string
		signal syscall.Signal
	}{
		{"INT", syscall.SIGINT},
		{"TERM", syscall.SIGTERM},
		{"HUP", sighup},
	} {
		t.Run("should pass SIG"+scenario.name+" on to the CLI and then end the program when only the program gets it",
			func(t *testing.T) {
				t.Parallel()

				// given
				skipUnlessUnix(t)
				if signal.Ignored(scenario.signal) {
					t.Skipf("this process ignores SIG%s, so the consumer would, and its shell cannot trap it",
						scenario.name)
				}
				directory := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(directory, stubCLIFile), []byte(stubCLIScript), 0o600))
				consumer, consumerOutput := startSignalConsumer(t, directory)
				stubPID := waitForStubPID(t, filepath.Join(directory, stubPIDFile), consumerOutput)

				// when
				// The consumer alone is signalled, as a terminal's interrupt now reaches only
				// the program: the CLI leads a process group of its own.
				require.NoError(t, consumer.Process.Signal(scenario.signal))
				err := consumer.Wait()

				// then
				assertDiedOf(t, err, scenario.signal, consumerOutput)
				marker, readErr := os.ReadFile(filepath.Join(directory, stubMarkerFile))
				require.NoError(t, readErr, "the stub CLI never got the signal")
				assert.Equal(t, scenario.name, string(marker))
				assert.False(t, processExists(stubPID), "the stub CLI outlived the consumer")
			})
	}
}

// skipUnlessUnix skips a test of behaviour that needs Unix process groups.
func skipUnlessUnix(t *testing.T) {
	t.Helper()
	if slices.Contains([]string{"windows", "plan9", "js", "wasip1"}, runtime.GOOS) {
		t.Skip("the signal relay needs Unix process groups")
	}
}

// startSignalConsumer starts this test binary in consumer mode on the stub CLI in directory,
// and returns it with the buffer its output goes to. The consumer is killed when the test
// ends, if it is still running.
func startSignalConsumer(t *testing.T, directory string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	consumer := exec.CommandContext(ctx, executable, "-test.run=^$")
	consumer.Env = append(os.Environ(), signalConsumerVariable+"="+directory)
	var output bytes.Buffer
	consumer.Stdout = &output
	consumer.Stderr = &output
	require.NoError(t, consumer.Start())
	return consumer, &output
}

// waitForStubPID waits for the stub CLI to write its process ID, which it does once its traps
// are set and the process that sleeps is running, and returns it. The stub is killed when the
// test ends, if it is still running.
func waitForStubPID(t *testing.T, path string, consumerOutput *bytes.Buffer) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(path); err == nil {
			if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(content))); parseErr == nil && pid > 0 {
				t.Cleanup(func() { killProcess(pid) })
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.FailNow(t, "the stub CLI never started", "consumer output: %s", consumerOutput.String())
	return 0
}

// assertDiedOf asserts that the consumer whose Wait returned err died of the given signal.
func assertDiedOf(t *testing.T, err error, expected syscall.Signal, consumerOutput *bytes.Buffer) {
	t.Helper()
	exitErr, exited := errors.AsType[*exec.ExitError](err)
	require.True(t, exited, "the consumer did not die of a signal (%v): %s", err, consumerOutput.String())
	status, isWaitStatus := exitErr.Sys().(syscall.WaitStatus)
	require.True(t, isWaitStatus, "unexpected process state %T", exitErr.Sys())
	require.True(t, status.Signaled(), "the consumer exited with status %d instead of dying of %v: %s",
		status.ExitStatus(), expected, consumerOutput.String())
	assert.Equal(t, expected, status.Signal())
}

// processExists reports whether a process with the given ID is still running.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer func() { _ = process.Release() }()
	return !errors.Is(process.Signal(syscall.Signal(0)), os.ErrProcessDone)
}

// killProcess kills the process with the given ID when it is still running, for a test that
// failed before that process could end.
func killProcess(pid int) {
	if !processExists(pid) {
		return
	}
	if process, err := os.FindProcess(pid); err == nil {
		_ = process.Kill()
		_ = process.Release()
	}
}
