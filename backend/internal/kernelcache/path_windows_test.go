//go:build windows

package kernelcache

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCacheAcceptsWindowsShortPathSpelling(t *testing.T) {
	root := t.TempDir()
	pointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	n, err := windows.GetShortPathName(pointer, &buffer[0], uint32(len(buffer)))
	if err != nil || n == 0 || int(n) >= len(buffer) {
		t.Fatalf("short path API: %v", err)
	}
	short := windows.UTF16ToString(buffer[:n])
	long, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(short, long) || !safeAncestors(filepath.Join(short, "future-cache")) {
		t.Fatalf("same directory rejected: short=%q long=%q", short, long)
	}
	if short == long {
		t.Log("filesystem returned identical spelling; CI runner temp aliases provide the distinct-spelling regression")
	}
}

func TestWindowsPathSpellingSupportsLongCachePaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), strings.Repeat("a", 80), strings.Repeat("b", 80), strings.Repeat("c", 80))
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	long, ok := longPathSpelling(root)
	if !ok || !samePath(root, long) {
		t.Fatal("long path spelling unavailable")
	}
}

func TestWindowsSpellingNormalizationDoesNotAdoptJunctionRoot(t *testing.T) {
	root := t.TempDir()
	target, link := filepath.Join(root, "target"), filepath.Join(root, "alias")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	// A test-owned junction requires no symlink privilege or machine setting.
	// Reject shell metacharacters before quoting the two literal fixture paths.
	if strings.ContainsAny(root, "&|<>^%!\r\n\"") {
		t.Skip("fixture path cannot be safely passed to Windows mklink")
	}
	if out, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("test-owned junction: %v / %s", err, out)
	}
	m := &Manager{}
	if m.safeDirectory(link) || safeAncestors(filepath.Join(link, "future-cache")) || samePath(link, target) {
		t.Fatal("path spelling fix adopted a reparse target")
	}
}
