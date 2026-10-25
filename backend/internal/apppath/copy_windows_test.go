//go:build windows

package apppath

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsFailedDataCopyIsNeverPublished(t *testing.T) {
	base := t.TempDir()
	src, dst := filepath.Join(base, "source"), filepath.Join(base, "destination")
	writeFixture(t, src, "a-first", "first fixture")
	writeFixture(t, src, "z-locked", "locked fixture")
	path, err := windows.UTF16PtrFromString(filepath.Join(src, "z-locked"))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyDirIfMissing(src, dst); err == nil {
		windows.CloseHandle(handle)
		t.Fatal("locked source was copied")
	}
	windows.CloseHandle(handle)
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("partial data was published")
	}
	assertFixture(t, src, "a-first", "first fixture")
	assertFixture(t, src, "z-locked", "locked fixture")
}
