//go:build !windows

package apppath

func registeredWindowsInstallRoot(string) bool { return false }

func ownsWindowsLegacyState(string) bool { return false }
