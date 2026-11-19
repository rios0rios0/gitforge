//go:build !unix

package infrastructure

import "os/exec"

// killProcessGroupOnCancel keeps exec's default on systems without Unix process groups:
// the context's end kills the CLI's own process.
func killProcessGroupOnCancel(_ *exec.Cmd) {}
