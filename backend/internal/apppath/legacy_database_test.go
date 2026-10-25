package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsConfiguredDatabaseAndSidecarsFollowSavedPath(t *testing.T) {
	for _, setting := range []string{"app.db", "storage/custom.sqlite", "profiles/app.db", "data/custom.sqlite", "bin/custom.sqlite", "../outside.sqlite", "absolute"} {
		t.Run(setting, func(t *testing.T) {
			root, state := windowsFixture(t)
			if setting == "absolute" {
				setting = filepath.Join(t.TempDir(), "external.sqlite")
			}
			source, target := filepath.Join(root, setting), filepath.Join(state, setting)
			if filepath.IsAbs(setting) {
				source, target = setting, setting
			} else if setting == "../outside.sqlite" {
				target = source
			}
			for _, suffix := range legacyDatabaseSuffixes {
				writeFixture(t, filepath.Dir(source), filepath.Base(source)+suffix, "offline SQLite fixture"+suffix)
			}
			raw := "database:\n  sqlite:\n    path: '" + setting + "'\nbrowser:\n  user_data_root: profiles\n"
			writeFixture(t, root, "config.yaml", raw)
			writeFixture(t, root, "profiles/A/Preferences", "profile fixture")
			if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
				t.Fatal(err)
			}
			if got := resolveDatabasePathForOS(root, setting, "windows"); got != target {
				t.Fatalf("database path = %s, want %s", got, target)
			}
			for _, suffix := range legacyDatabaseSuffixes {
				assertFixture(t, filepath.Dir(source), filepath.Base(source)+suffix, "offline SQLite fixture"+suffix)
				assertFixture(t, filepath.Dir(target), filepath.Base(target)+suffix, "offline SQLite fixture"+suffix)
			}
			assertFixture(t, state, "profiles/A/Preferences", "profile fixture")
			assertFixture(t, state, "config.yaml", raw)
			writeFixture(t, filepath.Dir(target), filepath.Base(target), "saved user database")
			if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
				t.Fatal(err)
			}
			assertFixture(t, filepath.Dir(target), filepath.Base(target), "saved user database")
		})
	}
}

func TestWindowsConfiguredDatabaseRefusesForeignOrMixedFiles(t *testing.T) {
	for _, scenario := range []string{"foreign-main", "foreign-wal", "orphan-target-wal", "existing-user-database"} {
		t.Run(scenario, func(t *testing.T) {
			root, state := windowsFixture(t)
			writeFixture(t, root, "config.yaml", "database:\n  sqlite:\n    path: app.db\n")
			writeFixture(t, root, "app.db", "legacy database")
			writeFixture(t, root, "app.db-wal", "legacy WAL")
			windowsLegacyFileOwned = func(path string) bool {
				return !(scenario == "foreign-main" && path == filepath.Join(root, "app.db")) &&
					!(scenario == "foreign-wal" && path == filepath.Join(root, "app.db-wal"))
			}
			if scenario == "existing-user-database" {
				writeFixture(t, state, "app.db", "current database")
				writeFixture(t, state, "app.db-wal", "current WAL")
				if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
					t.Fatal(err)
				}
				assertFixture(t, state, "app.db", "current database")
				assertFixture(t, state, "app.db-wal", "current WAL")
				return
			}
			if scenario == "orphan-target-wal" {
				writeFixture(t, state, "app.db-wal", "keep orphan evidence")
			}
			if err := ensureWritableLayoutForOS(root, "windows"); err == nil {
				t.Fatal("unsafe database bundle was adopted")
			}
			for _, name := range []string{"app.db", "config.yaml"} {
				if _, err := os.Stat(filepath.Join(state, name)); !os.IsNotExist(err) {
					t.Fatal("refused database migration was published")
				}
			}
			assertFixture(t, root, "app.db", "legacy database")
			assertFixture(t, root, "app.db-wal", "legacy WAL")
		})
	}
}
