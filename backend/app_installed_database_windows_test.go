package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsConfiguredSQLiteMigrationRecoversWALAndKeepsReopenState(t *testing.T) {
	root, state := installedCompatibilityFixture(t)
	cfg := config.DefaultConfig()
	cfg.Database.SQLite.Path = "app.db"
	if err := cfg.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	setFixtureOwnerToCurrentUser(t, filepath.Join(root, "config.yaml"))
	// Create a stable offline crash fixture: committed profile/proxy rows live
	// in WAL, then copy its bytes while no further SQL is being issued. The
	// migration source itself has no open database connection or service.
	payloadDB := filepath.Join(t.TempDir(), "fixture.db")
	db, err := database.NewDB(payloadDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.GetConn().Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetConn().Exec(`INSERT INTO browser_profiles(profile_id,profile_name,created_at,updated_at) VALUES('saved-profile','saved profile','fixture','fixture')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetConn().Exec(`INSERT INTO browser_proxies(proxy_id,proxy_name,proxy_config) VALUES('saved-proxy','saved proxy','')`); err != nil {
		t.Fatal(err)
	}
	before := make(map[string]string)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(payloadDB + suffix)
		if err != nil || len(data) == 0 {
			t.Fatal("offline SQLite/WAL fixture is incomplete")
		}
		path := filepath.Join(root, "app.db") + suffix
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		setFixtureOwnerToCurrentUser(t, path)
		before[path] = sha256Hex(data)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeLayout(root); err != nil {
		t.Fatal(err)
	}
	a := &App{appState: &appState{appRoot: root}}
	a.config = a.startupLoadConfig()
	userDB, err := a.startupInitDatabase(a.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = userDB.Close() })
	var name string
	if err := userDB.GetConn().QueryRow(`SELECT profile_name FROM browser_profiles WHERE profile_id='saved-profile'`).Scan(&name); err != nil || name != "saved profile" {
		t.Fatal("configured database opened empty or lost WAL profile data")
	}
	if err := userDB.GetConn().QueryRow(`SELECT proxy_name FROM browser_proxies WHERE proxy_id='saved-proxy'`).Scan(&name); err != nil || name != "saved proxy" {
		t.Fatal("configured database lost proxy data")
	}
	if a.backupResolveDBPath(a.config) != filepath.Join(state, "app.db") {
		t.Fatal("backup used a different configured database")
	}
	scope, err := a.backupBuildScope()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range scope.Entries {
		if entry.ID == "database_sqlite_main" {
			found = entry.SourcePath == filepath.Join(state, "app.db")
		}
	}
	if !found {
		t.Fatal("backup scope lost the database installation context")
	}
	if _, err := userDB.GetConn().Exec(`UPDATE browser_profiles SET profile_name='saved after migration' WHERE profile_id='saved-profile'`); err != nil {
		t.Fatal(err)
	}
	if err := userDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRuntimeLayout(root); err != nil {
		t.Fatal(err)
	}
	reopened, err := a.startupInitDatabase(a.config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.GetConn().QueryRow(`SELECT profile_name FROM browser_profiles WHERE profile_id='saved-profile'`).Scan(&name); err != nil || name != "saved after migration" {
		t.Fatal("reopen overwrote current user data")
	}
	for path, hash := range before {
		data, err := os.ReadFile(path)
		if err != nil || sha256Hex(data) != hash {
			t.Fatal("legacy SQLite source or sidecar changed")
		}
	}
}
