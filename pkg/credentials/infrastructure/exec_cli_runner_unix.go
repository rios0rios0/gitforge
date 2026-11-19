//go:build unix

package infrastructure

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

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
