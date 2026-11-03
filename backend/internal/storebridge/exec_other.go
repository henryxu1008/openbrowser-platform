//go:build !windows

package storebridge

import (
	"context"
	"os"
	"os/exec"
)

// Preserve a closed capability until equivalent OS ownership proof is provided.
func listenerOwnershipSupported() bool            { return false }
func safeRuntimeAncestors(string) bool            { return false }
func hideOwnedProcess(*exec.Cmd)                  {}
func openRuntime(path string) (*os.File, error)   { return os.Open(path) }
func ownsListener(context.Context, int, int) bool { return false }
