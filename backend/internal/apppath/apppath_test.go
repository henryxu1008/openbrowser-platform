package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

func windowsFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	programs := filepath.Join(base, "Program Files")
	root := filepath.Join(programs, "Ant Browser")
	user := filepath.Join(base, "user", "Local")
	for _, key := range []string{"ProgramW6432", "ProgramFiles", "ProgramFiles(x86)"} {
		t.Setenv(key, programs)
	}
	t.Setenv("LOCALAPPDATA", user)
	previous := windowsLegacyStateOwned
	windowsLegacyStateOwned = func(string) bool { return true }
	t.Cleanup(func() { windowsLegacyStateOwned = previous })
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(user, appStateDirName)
}

func writeFixture(t *testing.T, root, path, value string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertFixture(t *testing.T, root, path, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil || string(data) != want {
		t.Fatalf("fixture %s changed: %q, %v", path, data, err)
	}
}

func TestWindowsInstalledStatePreservesDataAcrossOpenAndUpgrade(t *testing.T) {
	root, state := windowsFixture(t)
	fixtures := map[string]string{"config.yaml": "legacy config", "proxies.yaml": "legacy proxies", "data/app.db": "SQLite fixture", "data/profiles/A/Preferences": "profile fixture", "chrome/148.0.7778.215/chrome.exe": "kernel fixture"}
	for path, value := range fixtures {
		writeFixture(t, root, path, value)
	}
	if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
		t.Fatal(err)
	}
	if got := detectForOS(root, "windows"); !got.detached || got.stateRoot != state {
		t.Fatalf("installed root not detached: %+v", got)
	}
	for path, value := range fixtures {
		assertFixture(t, root, path, value)
		assertFixture(t, state, path, value)
	}
	if got := resolveForOS(root, "bin/xray.exe", "windows"); got != filepath.Join(root, "bin", "xray.exe") {
		t.Fatalf("bundled proxy runtime moved: %s", got)
	}
	if got := resolveForOS(root, "data/app.db", "windows"); got != filepath.Join(state, "data", "app.db") {
		t.Fatalf("database path: %s", got)
	}
	writeFixture(t, state, "data/app.db", "saved user state")
	writeFixture(t, state, "config.yaml", "saved user config")
	writeFixture(t, root, "config.yaml", "new release defaults")
	if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
		t.Fatal(err)
	}
	assertFixture(t, state, "data/app.db", "saved user state")
	assertFixture(t, state, "config.yaml", "saved user config")
	assertFixture(t, root, "data/app.db", "SQLite fixture")
}

func TestWindowsRegisteredCustomInstallAndPortableRoots(t *testing.T) {
	_, state := windowsFixture(t)
	custom := t.TempDir()
	previous := windowsInstalledRoot
	windowsInstalledRoot = func(root string) bool { return root == custom }
	t.Cleanup(func() { windowsInstalledRoot = previous })
	if got := detectForOS(custom, "windows"); !got.detached || got.stateRoot != state {
		t.Fatalf("registered custom install: %+v", got)
	}
	portable := t.TempDir()
	writeFixture(t, portable, "data/app.db", "portable data")
	if err := ensureWritableLayoutForOS(portable, "windows"); err != nil {
		t.Fatal(err)
	}
	if got := detectForOS(portable, "windows"); got.detached || got.stateRoot != portable {
		t.Fatalf("portable root adopted installed data: %+v", got)
	}
	assertFixture(t, portable, "data/app.db", "portable data")
	// Different installation paths for one account must resolve the same state.
	second := filepath.Join(os.Getenv("ProgramFiles"), "Ant Browser Updated")
	if got := detectForOS(second, "windows").stateRoot; got != state {
		t.Fatalf("upgrade selected a different user root: %s", got)
	}
}

func TestWindowsFreshInstallAndCopyRefusalKeepExistingData(t *testing.T) {
	root, state := windowsFixture(t)
	writeFixture(t, root, "config.yaml", "defaults")
	if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "data")); err != nil {
		t.Fatal(err)
	}
	// An existing destination is authoritative; it is never overwritten.
	writeFixture(t, root, "data/app.db", "later legacy database")
	writeFixture(t, state, "data/app.db", "user database")
	if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
		t.Fatal(err)
	}
	assertFixture(t, state, "data/app.db", "user database")
	blocked := filepath.Join(t.TempDir(), "file-not-directory")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyDirIfMissing(filepath.Join(root, "data"), filepath.Join(blocked, "data")); err == nil {
		t.Fatal("copy error hidden")
	}
	assertFixture(t, root, "data/app.db", "later legacy database")
}

func TestWindowsDoesNotAdoptAnotherUsersLegacyProfiles(t *testing.T) {
	root, state := windowsFixture(t)
	writeFixture(t, root, "config.yaml", "other user configuration")
	writeFixture(t, root, "proxies.yaml", "other user proxy configuration")
	writeFixture(t, root, "data/app.db", "other user database")
	writeFixture(t, root, "config.defaults.yaml", "publisher defaults")
	windowsLegacyStateOwned = func(string) bool { return false }
	if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
		t.Fatal(err)
	}
	assertFixture(t, state, "config.yaml", "publisher defaults")
	for _, path := range []string{"proxies.yaml", "data/app.db"} {
		if _, err := os.Stat(filepath.Join(state, path)); !os.IsNotExist(err) {
			t.Fatalf("adopted foreign legacy state: %s", path)
		}
	}
	assertFixture(t, root, "data/app.db", "other user database")
	assertFixture(t, root, "config.yaml", "other user configuration")
}
