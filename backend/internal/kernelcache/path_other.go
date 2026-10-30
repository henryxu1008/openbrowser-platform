//go:build !windows

package kernelcache

func longPathSpelling(value string) (string, bool) { return value, true }
