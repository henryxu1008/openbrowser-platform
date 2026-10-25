package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsInstalledExtensionPathsUseCopiedArtifactsAndPreserveBoundary(t *testing.T) {
	root, state := installedCompatibilityFixture(t)
	id := backupTestExtensionID
	oldInstall := filepath.Join(root, "data", "extensions", id)
	oldPackage := filepath.Join(root, "data", "extensions", "packages", id+".crx")
	oldBackup := filepath.Join(root, "data", "extension-backups", backupTestProfileID, id, backupTestTimestamp)
	payload := writeBackupExtensionPayload(t, t.TempDir(), id, []byte(backupTestPackageData), true, oldBackup)
	if err := os.Rename(filepath.Join(payload, "app", "data"), filepath.Join(root, "data")); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	if err := cfg.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	sourceDB := filepath.Join(root, "data", "app.db")
	db, err := database.NewDB(sourceDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	legacy := &App{appState: &appState{appRoot: root, config: cfg, db: db}}
	insertBackupTestExtension(t, legacy, id, oldInstall, oldPackage, "legacy-hash")
	insertBackupTestRuntime(t, legacy, id, oldBackup, "runtime-hash")
	// A second record has external paths with a deceptively similar prefix.
	// Neither startup nor deletion may treat these as installation-managed.
	external := filepath.Join(root, "data", "extensions-other", backupTestExtensionIDB)
	insertBackupTestExtension(t, legacy, backupTestExtensionIDB, external, external+".crx", "external-hash")
	insertBackupTestRuntime(t, legacy, backupTestExtensionIDB, external, "external-runtime")
	mixedID := "cccccccccccccccccccccccccccccccc"
	mixedInstall := filepath.Join(root, "data", "extensions", mixedID)
	if err := os.MkdirAll(mixedInstall, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mixedInstall, "manifest.json"), []byte(`{"name":"Mixed","version":"1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	insertBackupTestExtension(t, legacy, mixedID, mixedInstall, external+".crx", "external-hash")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	setFixtureOwnerToCurrentUser(t, sourceDB)
	before, err := os.ReadFile(sourceDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeLayout(root); err != nil {
		t.Fatal(err)
	}
	userDB, err := database.NewDB(filepath.Join(state, "data", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = userDB.Close() })
	a := &App{appState: &appState{appRoot: root, config: cfg, db: userDB}}
	if err := userDB.Migrate(); err != nil {
		t.Fatal(err)
	}
	// Revocation in the check/commit gap must roll back every metadata update.
	revoked := errors.New("offline admission revoked")
	checks := 0
	if err := a.startupRepairInstalledExtensionPaths(func() error {
		checks++
		if checks == 2 {
			return revoked
		}
		return nil
	}); !errors.Is(err, revoked) || checks != 2 {
		t.Fatal("commit did not honor admission revocation")
	}
	if install, pkg, hash := readBackupTestExtension(t, a, id); install != oldInstall || pkg != oldPackage || hash != "legacy-hash" {
		t.Fatal("revoked repair partially committed")
	}
	check := func() error { return nil }
	if err := a.startupRepairInstalledExtensionPaths(check); err != nil {
		t.Fatal(err)
	}
	wantInstall := filepath.Join(state, "data", "extensions", id)
	wantPackage := filepath.Join(state, "data", "extensions", "packages", id+".crx")
	wantBackup := filepath.Join(state, "data", "extension-backups", backupTestProfileID, id, backupTestTimestamp)
	extension, err := browser.NewSQLiteExtensionDAO(userDB.GetConn()).Get(id)
	if err != nil || extension.InstallDir != wantInstall || extension.PackagePath != wantPackage || extension.PackageHash != sha256Hex([]byte(backupTestPackageData)) {
		t.Fatal("extension DAO still uses the installation original")
	}
	if data, err := os.ReadFile(extension.PackagePath); err != nil || string(data) != backupTestPackageData {
		t.Fatal("runtime package lookup did not reach the copied package")
	}
	if got, err := a.resolveBrowserExtensionInstallDir(extension.InstallDir); err != nil || got != wantInstall {
		t.Fatal("copied extension failed the existing deletion boundary")
	}
	for _, outside := range []string{oldInstall, external, filepath.Join(state, "data", "extensions")} {
		if _, err := a.resolveBrowserExtensionInstallDir(outside); err == nil {
			t.Fatal("extension deletion boundary was weakened")
		}
	}
	var gotBackup string
	if err := userDB.GetConn().QueryRow(`SELECT backup_path FROM browser_profile_extension_runtime WHERE profile_id=? AND extension_id=?`, backupTestProfileID, id).Scan(&gotBackup); err != nil || gotBackup != wantBackup {
		t.Fatal("runtime backup did not move to the copied tree")
	}
	if install, pkg, hash := readBackupTestExtension(t, a, backupTestExtensionIDB); install != external || pkg != external+".crx" || hash != "external-hash" {
		t.Fatal("external extension paths changed")
	}
	if install, pkg, hash := readBackupTestExtension(t, a, mixedID); install != filepath.Join(state, "data", "extensions", mixedID) || pkg != external+".crx" || hash != "external-hash" {
		t.Fatal("mixed record repair changed an unrelated external field")
	}
	if err := userDB.GetConn().QueryRow(`SELECT backup_path FROM browser_profile_extension_runtime WHERE profile_id=? AND extension_id=?`, backupTestProfileID, backupTestExtensionIDB).Scan(&gotBackup); err != nil || gotBackup != external {
		t.Fatal("external runtime backup path changed")
	}
	var updatedBefore, updatedAfter string
	_ = userDB.GetConn().QueryRow(`SELECT updated_at FROM browser_extensions WHERE extension_id=?`, id).Scan(&updatedBefore)
	if err := a.startupRepairInstalledExtensionPaths(check); err != nil {
		t.Fatal(err)
	}
	_ = userDB.GetConn().QueryRow(`SELECT updated_at FROM browser_extensions WHERE extension_id=?`, id).Scan(&updatedAfter)
	if updatedBefore == "" || updatedBefore != updatedAfter {
		t.Fatal("reopen rewrote already-migrated metadata")
	}
	after, err := os.ReadFile(sourceDB)
	if err != nil || sha256Hex(before) != sha256Hex(after) {
		t.Fatal("legacy source database was modified")
	}
	for _, original := range []string{filepath.Join(oldInstall, "manifest.json"), oldPackage, filepath.Join(oldBackup, "state.bin")} {
		if _, err := os.Stat(original); err != nil {
			t.Fatal("legacy extension original was removed")
		}
	}
}
