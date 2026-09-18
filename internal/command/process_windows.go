package command

import (
	"os"
	"os/exec"
)

// Windows has no process groups to inherit here, so the command is left alone
// and only the process itself is killed, as before.
func setProcessGroup(cmd *exec.Cmd) {}

func terminateProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Kill()
}

func killProcessGroup(cmd *exec.Cmd) error {
	return terminateProcessGroup(cmd)
}
