//go:build !unix

package infrastructure

import "os/exec"

// killProcessGroupOnCancel keeps exec's default on systems without Unix process groups:
// the context's end kills the CLI's own process.
func killProcessGroupOnCancel(_ *exec.Cmd) {}

// runCLI starts cmd and waits for it. The CLI is not moved out of this process's group here,
// so a console's interrupt reaches it as it reaches this process, and there is nothing to
// relay.
func runCLI(cmd *exec.Cmd) error {
	return cmd.Run()
}
