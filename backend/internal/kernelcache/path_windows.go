//go:build windows

package kernelcache

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
)

// Spelling equivalence cannot authorize a symbolic link or junction, even if
// an OS spelling API were to return its target for an ancestor component.
func unlinkedComponents(value string) bool {
	for current := filepath.Clean(value); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if filepath.Dir(current) == current {
			return true
		}
	}
}

func longPathSpelling(value string) (string, bool) {
	if !filepath.IsAbs(value) || !unlinkedComponents(value) {
		return "", false
	}
	if !strings.HasPrefix(value, `\\?\`) {
		if strings.HasPrefix(value, `\\`) {
			value = `\\?\UNC\` + strings.TrimPrefix(value, `\\`)
		} else {
			value = `\\?\` + value
		}
	}
	pointer, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return "", false
	}
	buffer := make([]uint16, 260)
	for {
		size, err := windows.GetLongPathName(pointer, &buffer[0], uint32(len(buffer)))
		if err != nil || size == 0 || size > 32768 {
			return "", false
		}
		if int(size) < len(buffer) {
			result := windows.UTF16ToString(buffer[:size])
			if strings.HasPrefix(result, `\\?\UNC\`) {
				result = `\\` + strings.TrimPrefix(result, `\\?\UNC\`)
			} else {
				result = strings.TrimPrefix(result, `\\?\`)
			}
			return result, true
		}
		buffer = make([]uint16, int(size)+1)
	}
}
