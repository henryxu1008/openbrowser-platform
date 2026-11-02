//go:build !windows

package storebridge

import "os/exec"

func hideFixture(*exec.Cmd) {}
