// +build windows

package util

import (
	"os/exec"
	"syscall"
)

func PrepareRunProc(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
