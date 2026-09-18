//go:build !windows

package executor

import (
	"log"
	"os/exec"
	"syscall"
	"time"
)

// killGracePeriod is how long the process group gets to exit after SIGTERM.
const killGracePeriod = 10 * time.Second

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup signals the group and escalates to SIGKILL unless the
// command finishes first. done must be closed when the command was reaped,
// otherwise the pid may be reused by then and the SIGKILL would hit a
// different process group.
func terminateProcessGroup(cmd *exec.Cmd, done <-chan struct{}) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		pgid = cmd.Process.Pid
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		log.Printf("unable to terminate process group %d: %s", pgid, err)
		return
	}
	select {
	case <-done:
	case <-time.After(killGracePeriod):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
