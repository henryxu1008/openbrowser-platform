package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func installedCompatibilityFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	programs := filepath.Join(base, "Program Files")
	root := filepath.Join(programs, "Ant Browser")
	local := filepath.Join(base, "normal-user", "Local")
	t.Setenv("ProgramFiles", programs)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("APPDATA", filepath.Join(base, "normal-user", "Roaming"))
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(local, "ant-browser")
}

func writeCompatibilityCore(t *testing.T, directory string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(directory, "chrome.exe")
	if err := os.WriteFile(exe, []byte("offline core fixture; never executed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), []byte(`{"version":"148.0.7778.215"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestWindowsDetachedImportedCoreLocationsRemainUsable(t *testing.T) {
	root, state := installedCompatibilityFixture(t)
	legacyDir := filepath.Join(root, "custom-core")
	legacyExe := writeCompatibilityCore(t, legacyDir)
	cfg := config.DefaultConfig()
	m := browser.NewManager(cfg, root)
	legacy := browser.Core{CoreId: "legacy", CorePath: "custom-core"}
	if path, err := m.ResolveCoreExecutable(legacy); err != nil || path != legacyExe {
		t.Fatal("legacy installation-relative core was lost")
	}
	if !m.ValidateCorePath(legacy.CorePath).Valid || m.GetChromeVersion(legacy.CorePath) != "148.0.7778.215" {
		t.Fatal("core validation/version used a different directory")
	}
	a := &App{appState: &appState{appRoot: root, config: cfg, browserMgr: m}}
	path := a.relativeCorePathIfPossible(legacyDir)
	if path != legacyDir || !filepath.IsAbs(path) {
		t.Fatal("new installation-local import became ambiguous")
	}
	if err := m.SaveCore(browser.CoreInput{CoreId: "new-import", CoreName: "fixture", CorePath: path}); err != nil {
		t.Fatal(err)
	}
	if saved, ok := m.GetCore("new-import"); !ok {
		t.Fatal("core was not saved")
	} else if got, err := m.ResolveCoreExecutable(saved); err != nil || got != legacyExe {
		t.Fatal("saved import cannot be resolved")
	}
	managedDir := filepath.Join(state, "chrome", "managed")
	managedExe := writeCompatibilityCore(t, managedDir)
	managedPath := a.relativeCorePathIfPossible(managedDir)
	if filepath.IsAbs(managedPath) {
		t.Fatal("managed core stopped using state-relative paths")
	}
	if got, err := m.ResolveCoreExecutable(browser.Core{CorePath: managedPath}); err != nil || got != managedExe {
		t.Fatal("managed core moved back to installation")
	}
	preferredExe := writeCompatibilityCore(t, filepath.Join(state, "custom-core"))
	if got, err := m.ResolveCoreExecutable(legacy); err != nil || got != preferredExe {
		t.Fatal("existing user core did not retain precedence")
	}
}

func setFixtureOwnerToCurrentUser(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	// Only a file created inside this test's TempDir is passed here. No account,
	// IAM, global security policy, or real user file is changed.
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsLegacyBackupCredentialsReachNormalUserWithoutResurrection(t *testing.T) {
	root, state := installedCompatibilityFixture(t)
	cfg := config.DefaultConfig()
	if err := cfg.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	setFixtureOwnerToCurrentUser(t, filepath.Join(root, "config.yaml"))
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	settings := cfg.Backup
	settings.Channels.OpenList.Token = hex.EncodeToString(random[:])
	legacy := filepath.Join(root, backupLocalConfigFileName)
	if err := saveBackupLocalConfig(legacy, settings); err != nil {
		t.Fatal(err)
	}
	setFixtureOwnerToCurrentUser(t, legacy)
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(legacy, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(legacy, 0600)
	// The elevated installer may have preserved a copy in a different account.
	adminCopy := filepath.Join(t.TempDir(), "admin-roaming", backupLocalConfigDirName, backupLocalConfigFileName)
	if err := saveBackupLocalConfig(adminCopy, settings); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeLayout(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, backupLocalConfigFileName)); err != nil {
		t.Fatal("owned legacy credentials were not carried into user state")
	}
	a := &App{appState: &appState{appRoot: root, config: config.DefaultConfig()}}
	a.activateStableBackupLocalConfig()
	if err := a.prepareBackupLocalConfig(); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := loadBackupLocalConfig(a.backupLocalConfigPath(), cfg.Backup)
	if err != nil || !exists || loaded.Channels.OpenList.Token != settings.Channels.OpenList.Token {
		t.Fatal("normal user's stable backup credentials were not recovered")
	}
	after, err := os.ReadFile(legacy)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("installation original was changed or deleted")
	}
	if _, err := os.Stat(adminCopy); err != nil {
		t.Fatal("installer account's private copy was changed")
	}
	if err := a.removeBackupLocalConfigs(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeLayout(root); err != nil {
		t.Fatal(err)
	}
	reopened := &App{appState: &appState{appRoot: root, config: config.DefaultConfig()}}
	reopened.activateStableBackupLocalConfig()
	if err := reopened.prepareBackupLocalConfig(); err != nil {
		t.Fatal(err)
	}
	if _, exists, _, err := reopened.loadBackupLocalConfig(cfg.Backup); err != nil || exists {
		t.Fatal("cleared credentials were resurrected from the installation original")
	}
}
