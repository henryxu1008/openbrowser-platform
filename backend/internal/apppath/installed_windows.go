//go:build windows

package apppath

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"strings"
)

// A shared legacy install is not proof that its profiles belong to this user.
// Unknown/group/foreign ownership leaves the original data untouched for an
// explicit import using the existing native profile/backup flows.
func ownsWindowsLegacyState(root string) bool {
	legacyFile := filepath.Join(root, "data", "app.db")
	if cfg, err := config.Load(filepath.Join(root, "config.yaml")); err == nil {
		p := fsutil.NormalizePathInput(cfg.Database.SQLite.Path)
		if filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) {
			// External databases stay at their configured origin. The installed
			// config, rather than an unrelated data/app.db, owns legacy adoption.
			return windowsLegacyFileOwned(filepath.Join(root, "config.yaml"))
		}
		legacyFile = filepath.Join(root, p)
	}
	if _, err := os.Stat(legacyFile); os.IsNotExist(err) {
		legacyFile = filepath.Join(root, "config.yaml")
	} else if err != nil {
		return false
	}
	return windowsLegacyFileOwned(legacyFile)
}

func ownsWindowsFile(path string) bool {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	return err == nil && owner != nil && owner.Equals(user.User.Sid)
}

// Reuse the existing NSIS install registration, including older 32-bit views.
func registeredWindowsInstallRoot(root string) bool {
	for _, view := range []uint32{registry.WOW64_32KEY, registry.WOW64_64KEY} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall\AntBrowser`, registry.QUERY_VALUE|view)
		if err != nil {
			continue
		}
		location, _, err := key.GetStringValue("InstallLocation")
		key.Close()
		if err == nil && strings.TrimSpace(location) != "" && strings.EqualFold(normalizeRoot(location), root) {
			return true
		}
	}
	return false
}
