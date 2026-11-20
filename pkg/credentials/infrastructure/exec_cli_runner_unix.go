//go:build unix

package infrastructure

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// relayedSignals are the signals that ask a program to end: a terminal's interrupt, a request
// to terminate, and the hangup of its terminal.
//
//nolint:gochecknoglobals // read-only lookup table
var relayedSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}

// killProcessGroupOnCancel starts the CLI in a process group of its own and, when the
// context ends, kills the whole group. A launcher that runs the real program without exec
// (the Debian az script runs Python that way) would otherwise leave that program running
// after the deadline, holding standard output open.
func killProcessGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// A negative PID addresses the process group the child leads.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

// runCLI starts cmd, which leads a process group of its own (killProcessGroupOnCancel), and
// waits for it to exit. Being in another group, the CLI no longer receives the interrupt a
// terminal sends to this process's group, so a program stopped with Ctrl-C would exit and
// leave the CLI running, with nothing left to enforce its timeout. While the CLI runs, every
// relayed signal this process receives is therefore sent on to the CLI's group. Once the CLI
// has exited, the relay stops and the first signal received is sent to this process again, so
// the program ends, or runs its own handler, as it would have if the CLI shared its group.
//
// A signal this process ignores is not relayed: sharing the group, the CLI would have
// inherited the same disposition and ignored it too.
func runCLI(cmd *exec.Cmd) error {
	received := make(chan os.Signal, len(relayedSignals))
	for _, relayed := range relayedSignals {
		if !signal.Ignored(relayed) {
			signal.Notify(received, relayed)
		}
	}

	// The relay is listening before the CLI starts, so no signal falls between the two.
	if err := cmd.Start(); err != nil {
		redeliverSignal(stopRelay(received, nil))
		return err
	}

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	var first os.Signal
	for {
		select {
		case relayed := <-received:
			if first == nil {
				first = relayed
			}
			if number, isSyscall := relayed.(syscall.Signal); isSyscall {
				// A negative PID addresses the CLI's process group; the group may already be
				// gone, which leaves nothing to do.
				_ = syscall.Kill(-cmd.Process.Pid, number)
			}
		case err := <-waited:
			redeliverSignal(stopRelay(received, first))
			return err
		}
	}
}

// stopRelay stops the delivery of signals to received and returns first, or else the first
// signal that arrived before the delivery stopped, or nil when none did.
func stopRelay(received chan os.Signal, first os.Signal) os.Signal {
	signal.Stop(received)
	for {
		select {
		case late := <-received:
			if first == nil {
				first = late
			}
		default:
			return first
		}
	}
}

// redeliverSignal sends this process the signal it received while it was relaying, now that
// it no longer catches it, so the signal takes the effect it would have had. Nil sends none.
func redeliverSignal(received os.Signal) {
	if number, isSyscall := received.(syscall.Signal); isSyscall {
		_ = syscall.Kill(os.Getpid(), number)
	}
}
