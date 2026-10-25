//go:build windows

package apppath

import (
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

func TestWindowsLegacyCopyRejectsJunctions(t *testing.T) {
	base := t.TempDir()
	src, dst, other := filepath.Join(base, "source"), filepath.Join(base, "destination"), filepath.Join(base, "other")
	writeFixture(t, src, "a-first", "source fixture")
	writeFixture(t, other, "private", "do not adopt")
	command := exec.Command("cmd.exe", "/c", "mklink", "/J", filepath.Join(src, "z-link"), other)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create owned fixture junction: %v %s", err, output)
	}
	if err := copyDirIfMissing(src, dst); err == nil {
		t.Fatal("junction adopted outside data")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("partial data published after junction")
	}
	assertFixture(t, other, "private", "do not adopt")
}
