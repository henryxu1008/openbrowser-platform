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
func copyLegacyUserDataRoot(root roots, goos string) error {
	if _, err := os.Stat(filepath.Join(root.stateRoot, "config.yaml")); !os.IsNotExist(err) {
		return err
	}
	cfg, err := config.Load(filepath.Join(root.installRoot, "config.yaml"))
	if err != nil {
		return fmt.Errorf("read legacy profile root: %w", err)
	}
	p := fsutil.NormalizePathInput(cfg.Browser.UserDataRoot)
	if p == "" || filepath.IsAbs(p) {
		return nil
	}
	source := filepath.Join(root.installRoot, p)
	target := resolveUserDataRootForOS(root.installRoot, p, goos)
	if source == target {
		return nil // Absolute/external, bin/ and installation-root settings stay put.
	}
	// Walk also checks the copied root, but a configured nested root must not
	// traverse a junction/symlink in one of its parent components either.
	parent := root.installRoot
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
	return copyDirIfMissing(source, target)
}
