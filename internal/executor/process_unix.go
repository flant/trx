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

func terminateProcessGroup(cmd *exec.Cmd) {
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
	time.AfterFunc(killGracePeriod, func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	})
}
