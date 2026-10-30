package backend

import (
	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/internal/kernelapi"
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeKernelSnapshotPackage(t *testing.T, path string, profiles []browser.Profile) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	writeJSONToZip(t, w, "manifest.json", ProfilePackageManifest{Format: profilePackageFormat, Version: profilePackageVersion, ProfileCount: len(profiles), DatabaseVersion: profilePackageDatabaseVersion})
	writeJSONToZip(t, w, profilePackageDatabasePath, ProfilePackageDatabase{Format: profilePackageDatabaseFormat, Version: profilePackageDatabaseVersion, Profiles: profiles, Cores: []ProfilePackageDatabaseCore{{CoreId: "package-local-core", CoreName: "untrusted-local", CorePath: "Z:/not-a-platform-install", IsDefault: true}}})
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func previewKernelPackage(t *testing.T, a *App, path string) *App {
	t.Helper()
	scoped, err := a.scoped(7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.prepareProfilePackageImportFromPath(path); err != nil {
		t.Fatal(err)
	}
	return scoped
}

func TestKernelPackageNewAndOverwriteCannotBindWithdrawnRelease(t *testing.T) {
	for _, mode := range []string{profilePackageImportModeNew, profilePackageImportModeOverwrite} {
		t.Run(mode, func(t *testing.T) {
			a, b, _, source, id := preparedKernelAppFixture(t)
			original, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "target", CoreId: id}, 7)
			if err != nil {
				t.Fatal(err)
			}
			dir := a.browserMgr.ResolveUserDataDir(original)
			if err = os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(dir, "original.txt")
			if err = os.WriteFile(sentinel, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			name := "incoming"
			if mode == profilePackageImportModeOverwrite {
				name = "target"
			}
			path := filepath.Join(t.TempDir(), "owned.zip")
			writeTestProfilePackage(t, path, []browser.Profile{{ProfileId: "source-import", ProfileName: name, UserDataDir: "source-import", CoreId: id}}, map[string]string{"source-import/Default/Preferences": "{}"})
			previewKernelPackage(t, a, path)
			source.err = kernelapi.ErrNotFound
			_, err = b.BrowserProfilePackageImportWithOptions(path, ProfilePackageImportOptions{ConflictMode: mode, ConfirmConflict: true}, 7)
			if !errors.Is(err, kernelapi.ErrNotFound) {
				t.Fatalf("import created a withdrawn binding: %v", err)
			}
			if len(a.browserMgr.Profiles) != 1 {
				t.Fatal("rejected import changed profile map")
			}
			stored, err := a.browserMgr.ProfileDAO.GetById(original.ProfileId)
			if err != nil || stored.DeletedAt != "" || stored.CoreId != id {
				t.Fatal("rejected overwrite changed original binding")
			}
			data, err := os.ReadFile(sentinel)
			if err != nil || string(data) != "original" {
				t.Fatal("rejected import changed original data")
			}
		})
	}
}

func TestKernelSnapshotImportPreservesReleaseIDWithoutCoreDAOFact(t *testing.T) {
	a, b, _, _, id := preparedKernelAppFixture(t)
	path := filepath.Join(t.TempDir(), "owned-snapshot.zip")
	writeKernelSnapshotPackage(t, path, []browser.Profile{{ProfileId: "source-import", ProfileName: "snapshot", UserDataDir: "source-import", CoreId: id}})
	previewKernelPackage(t, a, path)
	result, err := b.BrowserProfilePackageImportWithOptions(path, ProfilePackageImportOptions{ConflictMode: "new", ConfirmConflict: true}, 7)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := a.browserMgr.ProfileDAO.GetById(result.ProfileMappings["source-import"])
	if err != nil || imported.CoreId != id {
		t.Fatalf("issued ID changed: %v / %v", imported, err)
	}
	var coreRows int
	if err = a.db.GetConn().QueryRow("SELECT COUNT(*) FROM browser_cores").Scan(&coreRows); err != nil {
		t.Fatal(err)
	}
	if coreRows != 0 {
		t.Fatal("package created a second CoreDAO installation fact")
	}
	if len(a.browserMgr.BrowserProcesses) != 0 {
		t.Fatal("import entered process lifecycle")
	}
}

func TestKernelPackageRejectsUnpreparedAndLocalOnlyBindings(t *testing.T) {
	for _, kind := range []string{"not-prepared", "local-only"} {
		t.Run(kind, func(t *testing.T) {
			a, b, _, source, id := preparedKernelAppFixture(t)
			if kind == "local-only" {
				id = "old-local-core"
			} else {
				id = "20000000-0000-0000-0000-000000000002"
				source.artifact.ReleaseID = id
			}
			path := filepath.Join(t.TempDir(), "owned.zip")
			writeTestProfilePackage(t, path, []browser.Profile{{ProfileId: "source-import", ProfileName: "incoming", UserDataDir: "source-import", CoreId: id}}, nil)
			previewKernelPackage(t, a, path)
			if _, err := b.BrowserProfilePackageImportWithOptions(path, ProfilePackageImportOptions{ConflictMode: "new", ConfirmConflict: true}, 7); err == nil {
				t.Fatal("unprepared/local-only import admitted")
			}
			if len(a.browserMgr.Profiles) != 0 {
				t.Fatal("rejected package wrote profiles")
			}
		})
	}
}

func TestKernelPackagePinsIdentityAndOverwriteTargetDuringMetadataRead(t *testing.T) {
	for _, kind := range []string{"identity", "target"} {
		t.Run(kind, func(t *testing.T) {
			a, b, auth, source, id := preparedKernelAppFixture(t)
			original, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "target", CoreId: id}, 7)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "owned.zip")
			writeTestProfilePackage(t, path, []browser.Profile{{ProfileId: "source-import", ProfileName: "target", UserDataDir: "source-import", CoreId: id}}, nil)
			previewKernelPackage(t, a, path)
			source.artifactStarted = make(chan struct{})
			source.artifactRelease = make(chan struct{})
			result := make(chan error, 1)
			go func() {
				_, err := b.BrowserProfilePackageImportWithOptions(path, ProfilePackageImportOptions{ConflictMode: "overwrite", ConfirmConflict: true}, 7)
				result <- err
			}()
			select {
			case <-source.artifactStarted:
			case err := <-result:
				t.Fatalf("import skipped metadata admission: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("import did not reach metadata")
			}
			if kind == "identity" {
				auth.replace()
			} else {
				_, err = a.browserMgr.UpdateChecked(original.ProfileId, BrowserProfileInput{ProfileName: "intervening", CoreId: id}, func() error { return nil })
				if err != nil {
					t.Fatal(err)
				}
			}
			close(source.artifactRelease)
			if err = <-result; err == nil {
				t.Fatal("stale import committed")
			}
			stored, err := a.browserMgr.ProfileDAO.GetById(original.ProfileId)
			if err != nil || stored.DeletedAt != "" {
				t.Fatal("stale overwrite retired original")
			}
			if len(a.browserMgr.Profiles) != 1 {
				t.Fatal("stale import changed profile map")
			}
		})
	}
}

func TestAutomationTemplateCopyUsesPublishedBindingOwner(t *testing.T) {
	a, b, _, source, id := preparedKernelAppFixture(t)
	original, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "template", CoreId: id}, 7)
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := a.scoped(7)
	if err != nil {
		t.Fatal(err)
	}
	script := automation.ScriptRecord{TargetConfig: automation.ScriptTargetConfig{Mode: "create", TemplateSelector: automation.ScriptTargetSelector{ProfileID: original.ProfileId}}}
	source.err = kernelapi.ErrNotFound
	if _, _, err = scoped.resolveAutomationScriptTarget(script); !errors.Is(err, kernelapi.ErrNotFound) {
		t.Fatalf("template copy bypassed publication: %v", err)
	}
	if len(a.browserMgr.Profiles) != 1 {
		t.Fatal("rejected template created profile")
	}
}

func TestPlatformPackageOverwriteKeepsV1ProtocolAndRollsBackOriginalBinding(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "sql-failure"}[fail], func(t *testing.T) {
			a, b, _, source, id := preparedKernelAppFixture(t)
			original, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "target", CoreId: id}, 7)
			if err != nil {
				t.Fatal(err)
			}
			originalDir := a.browserMgr.ResolveUserDataDir(original)
			if err = os.MkdirAll(originalDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(originalDir, "original.txt"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "owned-v1.zip")
			writeTestProfilePackage(t, path, []browser.Profile{{ProfileId: "incoming", ProfileName: "target", UserDataDir: "incoming", CoreId: id}}, map[string]string{"incoming/Default/Preferences": "synthetic"})
			previewKernelPackage(t, a, path)
			if fail {
				// The original synthetic row was written before this test-only trigger.
				_, err = a.db.GetConn().Exec("CREATE TRIGGER fail_import BEFORE INSERT ON browser_profiles BEGIN SELECT RAISE(ABORT, 'owned_fixture_failure'); END")
				if err != nil {
					t.Fatal(err)
				}
			}
			result, err := b.BrowserProfilePackageImportWithOptions(path, ProfilePackageImportOptions{ConflictMode: "overwrite", ConfirmConflict: true}, 7)
			if !fail {
				if err != nil || result.OverwrittenCount != 1 {
					t.Fatalf("published V1 overwrite: %v", err)
				}
				p, err := a.browserMgr.ProfileDAO.GetById(result.ProfileMappings["incoming"])
				if err != nil || p.CoreId != id || p.ProfileId == original.ProfileId {
					t.Fatal("overwrite did not retain issued ID with new profile UUID")
				}
				data, err := os.ReadFile(filepath.Join(a.browserMgr.ResolveUserDataDir(p), "Default", "Preferences"))
				if err != nil || string(data) != "synthetic" {
					t.Fatal("V1 user data not restored")
				}
			} else {
				if err == nil {
					t.Fatal("forced SQL failure was lost")
				}
				p, err := a.browserMgr.ProfileDAO.GetById(original.ProfileId)
				if err != nil || p.DeletedAt != "" || p.CoreId != id {
					t.Fatal("rollback did not restore original binding")
				}
				if len(a.browserMgr.Profiles) != 1 || a.browserMgr.Profiles[original.ProfileId] == nil {
					t.Fatal("rollback changed original map")
				}
				data, err := os.ReadFile(filepath.Join(originalDir, "original.txt"))
				if err != nil || string(data) != "original" {
					t.Fatal("rollback lost original data")
				}
				source.err = kernelapi.ErrNotFound
				if _, err = a.db.GetConn().Exec("DROP TRIGGER fail_import"); err != nil {
					t.Fatal(err)
				}
				// Keeping the restored same-profile binding does not need republishing.
				if _, err = b.BrowserProfileUpdate(original.ProfileId, BrowserProfileInput{ProfileName: "still original", CoreId: id}, 7); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
