package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsInstalledCoreBackupRoundTripUsesManifestIDs(t *testing.T) {
	root, _ := installedCompatibilityFixture(t)
	external := t.TempDir()
	writeCompatibilityCore(t, filepath.Join(root, "chrome", "148"))
	writeCompatibilityCore(t, filepath.Join(root, "custom-core"))
	writeCompatibilityCore(t, external)
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{CoreId: "bundled", CorePath: "chrome/148"},
		{CoreId: "legacy", CorePath: "custom-core"},
		{CoreId: "absolute", CorePath: external},
	}
	if err := cfg.Save(filepath.Join(root, "config.defaults.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeLayout(root); err != nil {
		t.Fatal(err)
	}
	source := &App{appState: &appState{appRoot: root, config: cfg}}
	scope, err := source.backupBuildScope()
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "offline-backup.zip")
	manifest := backup.BuildManifest(scope, "offline fixture", "test", time.Unix(0, 0))
	if _, _, _, err := backupWritePackageZip(archive, scope, manifest, nil); err != nil {
		t.Fatal(err)
	}
	extracted, manifest, err := backupExtractAndValidate(archive)
	if err != nil {
		t.Fatal(err)
	}
	// Move only the newly created, verified temp extraction under this test's
	// own TempDir so standard test cleanup also owns every extracted file.
	rel, err := filepath.Rel(os.TempDir(), extracted)
	if err != nil || !filepath.IsAbs(extracted) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || !strings.HasPrefix(filepath.Base(extracted), "ant-chrome-import-") {
		t.Fatal("unexpected extraction ownership")
	}
	ownedExtraction := filepath.Join(t.TempDir(), "extracted")
	if err := os.Rename(extracted, ownedExtraction); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(ownedExtraction, "payload")
	incoming, exists, err := backupLoadIncomingConfig(payload)
	if err != nil || !exists {
		t.Fatal("archive lost the original core configuration")
	}
	targetRoot := t.TempDir()
	target := &App{appState: &appState{appRoot: targetRoot, config: config.DefaultConfig()}}
	incoming = target.backupNormalizeImportedConfigPaths(incoming, target.config)
	// Exercise the same ID mapping used when database merge renames a core.
	incoming.Browser.Cores[0].CoreId = "bundled-remapped"
	mappings := &backupImportReferenceMappings{Cores: map[string]backupImportIDMapping{"bundled": {TargetID: "bundled-remapped", Imported: true}}}
	stats := &backupMergeStats{}
	var issues []error
	target.backupImportFileTrees(payload, incoming, manifest, stats, func(_, _ string, err error) { issues = append(issues, err) }, mappings)
	if len(issues) != 0 || stats.Skipped != 0 {
		t.Fatalf("core restore skipped backed-up data: issues=%v skipped=%d", issues, stats.Skipped)
	}
	manager := browser.NewManager(incoming, targetRoot)
	for _, core := range incoming.Browser.Cores {
		exe, err := manager.ResolveCoreExecutable(core)
		if err != nil || !backupPathWithin(exe, targetRoot) {
			t.Fatalf("restored core %s still depends on the source machine: %v", core.CoreId, err)
		}
		if data, err := os.ReadFile(exe); err != nil || string(data) != "offline core fixture; never executed" {
			t.Fatal("restored core file differs from the archived fixture")
		}
	}
	if cfg.Browser.Cores[0].CorePath != "chrome/148" || cfg.Browser.Cores[1].CorePath != "custom-core" {
		t.Fatal("export rewrote the source configuration")
	}
}

func TestBackupManagedCoreTargetsRequireMatchingManifestID(t *testing.T) {
	a := &App{appState: &appState{appRoot: t.TempDir()}}
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{{CoreId: "managed", CorePath: "chrome/148"}, {CoreId: "external", CorePath: "custom-core"}}
	if targets := a.backupCollectExternalCoreTargets(cfg); len(targets) != 1 || targets[0].CoreId != "external" {
		t.Fatal("legacy unidentified target behavior changed")
	}
	targets := a.backupCollectExternalCoreTargets(cfg, map[string]struct{}{"managed": {}})
	used, index := map[string]struct{}{}, 0
	if path, ok := backupSelectExternalCoreTarget(targets, "", used, &index); !ok || path != filepath.Join(a.appRoot, "custom-core") {
		t.Fatal("unidentified payload selected a managed chrome target")
	}
	if _, ok := backupSelectExternalCoreTarget(targets, "unknown", used, &index); ok {
		t.Fatal("unknown payload fell back to a managed core")
	}
	if path, ok := backupSelectExternalCoreTarget(targets, "managed", used, &index); !ok || path != filepath.Join(a.appRoot, "chrome", "148") {
		t.Fatal("matching manifest ID did not restore its managed core")
	}
}
