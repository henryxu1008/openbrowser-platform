package apppath

import (
	"fmt"
	"os"
	"path/filepath"
)

var legacyDatabaseSuffixes = []string{"-wal", "-shm", "-journal", ""}

func prepareLegacyDatabaseCopy(installRoot, source, target string) (func(), bool, error) {
	noop := func() {}
	if source == target {
		return noop, false, nil
	}
	if _, err := os.Stat(target); err == nil {
		return noop, false, nil // Existing user database and sidecars are authoritative.
	} else if !os.IsNotExist(err) {
		return nil, false, err
	}
	for _, suffix := range legacyDatabaseSuffixes[:len(legacyDatabaseSuffixes)-1] {
		if _, err := os.Stat(target + suffix); !os.IsNotExist(err) {
			return nil, false, fmt.Errorf("refusing to mix existing database sidecar: %s", target+suffix)
		}
	}
	if _, err := os.Stat(source); os.IsNotExist(err) {
		for _, suffix := range legacyDatabaseSuffixes[:len(legacyDatabaseSuffixes)-1] {
			if _, err := os.Stat(source + suffix); !os.IsNotExist(err) {
				return nil, false, fmt.Errorf("legacy database missing beside sidecar: %s", source+suffix)
			}
		}
		return noop, false, nil
	} else if err != nil {
		return nil, false, err
	}
	rel, err := filepath.Rel(installRoot, filepath.Dir(source))
	if err != nil {
		return nil, false, err
	}
	if err := checkLegacyDirectoryPath(installRoot, rel); err != nil {
		return nil, false, err
	}
	files := make([]string, 0, len(legacyDatabaseSuffixes))
	for _, suffix := range legacyDatabaseSuffixes {
		path := source + suffix
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		if !info.Mode().IsRegular() || !windowsLegacyFileOwned(path) {
			return nil, false, fmt.Errorf("legacy database file is not owned regular data: %s", path)
		}
		files = append(files, path)
	}
	// The Windows implementation refuses active writers instead of copying a
	// changing SQLite/WAL set. Held read handles cover the complete tree copy too.
	release, err := holdLegacyDatabaseFiles(files)
	return release, err == nil, err
}

func copyLegacyDatabaseFiles(source, target string) error {
	if source == target {
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// Publish the database last. A failed sidecar copy must never expose an empty
	// or incomplete database as initialized. Partial copies remain as evidence.
	for _, suffix := range legacyDatabaseSuffixes {
		if err := copyFileIfMissing(source+suffix, target+suffix); err != nil {
			return err
		}
	}
	return nil
}
