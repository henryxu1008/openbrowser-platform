package apppath

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The caller has verified legacy ownership and holds the single-instance lock.
// Publish the config only after its relative profile tree is available; an
// existing user config marks completed initialization and must not reimport
// profiles that the user has subsequently removed.
func copyLegacyConfiguredPaths(root roots, goos string) error {
	if _, err := os.Stat(filepath.Join(root.stateRoot, "config.yaml")); !os.IsNotExist(err) {
		return err
	}
	cfg, err := config.Load(filepath.Join(root.installRoot, "config.yaml"))
	if err != nil {
		return fmt.Errorf("read legacy profile root: %w", err)
	}
	dbPath := fsutil.NormalizePathInput(cfg.Database.SQLite.Path)
	dbSource := filepath.Join(root.installRoot, dbPath)
	dbTarget := resolveDatabasePathForOS(root.installRoot, dbPath, goos)
	if filepath.IsAbs(dbPath) {
		dbSource = dbPath
	}
	release, copyDatabase, err := prepareLegacyDatabaseCopy(root.installRoot, dbSource, dbTarget)
	if err != nil {
		return err
	}
	defer release()
	// A configured database can be under chrome/ too. Freeze before this copy,
	// and copy cores before a nested profile root can create a partial parent.
	if err := copyDirIfMissing(filepath.Join(root.installRoot, "chrome"), filepath.Join(root.stateRoot, "chrome")); err != nil {
		return err
	}
	if err := copyDirIfMissing(filepath.Join(root.installRoot, "data"), filepath.Join(root.stateRoot, "data")); err != nil {
		return err
	}
	if err := copyLegacyUserDataRoot(root, cfg.Browser.UserDataRoot, goos); err != nil {
		return err
	}
	if copyDatabase {
		return copyLegacyDatabaseFiles(dbSource, dbTarget)
	}
	return nil
}

func copyLegacyUserDataRoot(root roots, setting, goos string) error {
	p := fsutil.NormalizePathInput(setting)
	if p == "" || filepath.IsAbs(p) {
		return nil
	}
	source := filepath.Join(root.installRoot, p)
	target := resolveUserDataRootForOS(root.installRoot, p, goos)
	if source == target {
		return nil // External and installation-root settings stay put.
	}
	if err := checkLegacyDirectoryPath(root.installRoot, p); err != nil {
		return err
	}
	return copyDirIfMissing(source, target)
}

func checkLegacyDirectoryPath(installRoot, p string) error {
	// Walk also checks the copied root, but a configured nested root must not
	// traverse a junction/symlink in one of its parent components either.
	parent := installRoot
	for _, part := range strings.Split(p, string(filepath.Separator)) {
		parent = filepath.Join(parent, part)
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("cannot migrate non-directory profile root: %s", parent)
		}
	}
	return nil
}
