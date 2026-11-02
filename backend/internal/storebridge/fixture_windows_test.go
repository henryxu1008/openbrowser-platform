package storebridge

import (
	"os/exec"
	"syscall"
)

func hideFixture(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
