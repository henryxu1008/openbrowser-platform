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

func TestWindowsCustomProfileRootRejectsParentJunction(t *testing.T) {
	root, state := windowsFixture(t)
	other := t.TempDir()
	writeFixture(t, root, "config.yaml", "browser:\n  user_data_root: linked/profiles\n")
	writeFixture(t, other, "profiles/A/Preferences", "do not adopt")
	command := exec.Command("cmd.exe", "/c", "mklink", "/J", filepath.Join(root, "linked"), other)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create owned fixture junction: %v %s", err, output)
	}
	if err := ensureWritableLayoutForOS(root, "windows"); err == nil {
		t.Fatal("custom root traversed a parent junction")
	}
	for _, path := range []string{"config.yaml", "linked/profiles"} {
		if _, err := os.Stat(filepath.Join(state, path)); !os.IsNotExist(err) {
			t.Fatal("failed custom root migration was published")
		}
	}
	assertFixture(t, other, "profiles/A/Preferences", "do not adopt")
}

func TestWindowsConfiguredDatabaseRefusesActiveWriter(t *testing.T) {
	for _, dbPath := range []string{"app.db", "profiles/app.db", "chrome/app.db"} {
		t.Run(dbPath, func(t *testing.T) {
			root, state := windowsFixture(t)
			writeFixture(t, root, "config.yaml", "database:\n  sqlite:\n    path: "+dbPath+"\n")
			writeFixture(t, root, dbPath, "owned database fixture")
			name, err := windows.UTF16PtrFromString(filepath.Join(root, dbPath))
			if err != nil {
				t.Fatal(err)
			}
			handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(handle)
			if err := ensureWritableLayoutForOS(root, "windows"); err == nil {
				t.Fatal("database copy accepted an active writer")
			}
			for _, path := range []string{dbPath, "config.yaml"} {
				if _, err := os.Stat(filepath.Join(state, path)); !os.IsNotExist(err) {
					t.Fatal("busy database was published")
				}
			}
			assertFixture(t, root, dbPath, "owned database fixture")
		})
	}
}

func TestWindowsLegacyOwnershipFollowsConfiguredDatabase(t *testing.T) {
	root, _ := windowsFixture(t)
	writeFixture(t, root, "config.yaml", "database:\n  sqlite:\n    path: app.db\n")
	writeFixture(t, root, "app.db", "configured database")
	writeFixture(t, root, "data/app.db", "unrelated stale default")
	windowsLegacyFileOwned = func(path string) bool { return path == filepath.Join(root, "app.db") }
	if !ownsWindowsLegacyState(root) {
		t.Fatal("configured owned database was ignored")
	}
	windowsLegacyFileOwned = func(path string) bool { return path == filepath.Join(root, "data", "app.db") }
	if ownsWindowsLegacyState(root) {
		t.Fatal("an unrelated default database conferred legacy ownership")
	}
}

func TestWindowsChromeDatabaseChecksOwnershipBeforeAnyTreeCopy(t *testing.T) {
	for _, foreignSuffix := range []string{"", "-wal"} {
		t.Run("foreign"+foreignSuffix, func(t *testing.T) {
			root, state := windowsFixture(t)
			writeFixture(t, root, "config.yaml", "database:\n  sqlite:\n    path: chrome/app.db\n")
			writeFixture(t, root, "chrome/app.db", "legacy database")
			writeFixture(t, root, "chrome/app.db-wal", "legacy WAL")
			foreign := filepath.Join(root, "chrome", "app.db") + foreignSuffix
			windowsLegacyFileOwned = func(path string) bool { return path != foreign }
			if err := ensureWritableLayoutForOS(root, "windows"); err == nil {
				t.Fatal("chrome copy bypassed database ownership")
			}
			for _, path := range []string{"chrome", "config.yaml"} {
				if _, err := os.Stat(filepath.Join(state, path)); !os.IsNotExist(err) {
					t.Fatal("unverified chrome database was published")
				}
			}
			assertFixture(t, root, "chrome/app.db", "legacy database")
		})
	}
}
