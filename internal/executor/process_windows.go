//go:build windows

package executor

import (
	"log"
	"os/exec"
)

// ponytail: Windows has no process groups here, only the direct child is
// killed. Use a job object if trx ever has to manage Windows workloads.
func setProcessGroup(cmd *exec.Cmd) {}

func terminateProcessGroup(cmd *exec.Cmd, _ <-chan struct{}) {
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("unable to kill process %d: %s", cmd.Process.Pid, err)
	}
}
