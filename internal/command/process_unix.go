//go:build !windows

package command

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup puts the command in a process group of its own, so that the
// whole tree can be signaled: the workload is a child of sh, not sh itself.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup asks the whole group to stop. Killing sh alone left the
// actual workload running as an orphan, outside the execution lock, while the
// next scheduled run started a second deployment beside it.
func terminateProcessGroup(cmd *exec.Cmd) error {
	return signalProcessGroup(cmd, syscall.SIGTERM)
}

// killProcessGroup is the escalation for a group that ignored SIGTERM. os/exec
// only kills the direct child when the wait delay expires, which would leave
// such a workload running.
func killProcessGroup(cmd *exec.Cmd) error {
	return signalProcessGroup(cmd, syscall.SIGKILL)
}

func signalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		// The group is already gone, which is what was asked for.
		return os.ErrProcessDone
	}
	return err
}
