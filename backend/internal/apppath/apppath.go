package apppath

import (
	"ant-chrome/backend/internal/fsutil"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
)

const appStateDirName = "ant-browser"

type roots struct {
	installRoot string
	stateRoot   string
	detached    bool
}

var rootsCache sync.Map

var windowsInstalledRoot = registeredWindowsInstallRoot
var windowsLegacyStateOwned = ownsWindowsLegacyState
var windowsLegacyFileOwned = ownsWindowsFile

// InstallRoot 返回应用安装根目录的绝对路径。
func InstallRoot(appRoot string) string {
	return detect(appRoot).installRoot
}

// StateRoot 返回应用可写状态目录的绝对路径。
func StateRoot(appRoot string) string {
	return detect(appRoot).stateRoot
}

// IsDetached 返回当前是否启用了“安装目录只读、状态目录独立”的模式。
func IsDetached(appRoot string) bool {
	return detect(appRoot).detached
}

// Resolve 将相对路径解析到安装目录或用户状态目录。
// 已安装应用启用 detached 模式后，除 bin/ 外的相对路径落到用户可写目录。
func Resolve(appRoot, p string) string {
	return resolveForOS(appRoot, p, goruntime.GOOS)
}

// ResolveUserDataRoot keeps relative roots outside the installation at their
// original location. Only installation-owned subdirectories are relocated.
func ResolveUserDataRoot(appRoot, p string) string {
	return resolveUserDataRootForOS(appRoot, p, goruntime.GOOS)
}

func ResolveDatabasePath(appRoot, p string) string {
	return resolveDatabasePathForOS(appRoot, p, goruntime.GOOS)
}

func resolveDatabasePathForOS(appRoot, p, goos string) string {
	if strings.TrimSpace(p) == "" {
		p = "data/app.db"
	}
	return resolveUserDataRootForOS(appRoot, p, goos)
}

func resolveUserDataRootForOS(appRoot, p, goos string) string {
	if strings.TrimSpace(p) == "" {
		p = "data"
	}
	p = fsutil.NormalizePathInput(p)
	if p == "" {
		return detectForOS(appRoot, goos).installRoot
	}
	if filepath.IsAbs(p) {
		return p
	}
	root := detectForOS(appRoot, goos)
	base := root.installRoot
	if root.detached && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator)) {
		// These are configured data paths, including e.g. bin/profiles. The
		// generic resolver's read-only bundled bin/ exception does not apply.
		base = root.stateRoot
	}
	return filepath.Join(base, p)
}

func resolveForOS(appRoot, p, goos string) string {
	p = fsutil.NormalizePathInput(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}

	root := detectForOS(appRoot, goos)
	base := root.installRoot
	if root.detached && useStateRoot(p) {
		base = root.stateRoot
	}
	return filepath.Join(base, p)
}

// EnsureWritableLayout 为需要 detached 状态目录的已安装应用准备首启所需的可写目录，
// 并把随包默认配置迁移到用户目录。
func EnsureWritableLayout(appRoot string) error {
	return ensureWritableLayoutForOS(appRoot, goruntime.GOOS)
}

func ensureWritableLayoutForOS(appRoot, goos string) error {
	root := detectForOS(appRoot, goos)
	if !root.detached {
		return nil
	}
	if root.stateRoot == "" {
		return fmt.Errorf("user data directory is unavailable")
	}

	if err := os.MkdirAll(root.stateRoot, 0755); err != nil {
		return err
	}
	// Transfer credentials before the first user config is published. After the
	// existing backup owner moves this copy to its stable directory, subsequent
	// starts must not resurrect credentials that the user has explicitly cleared.
	if normalizeGOOS(goos) == "windows" {
		_, configErr := os.Stat(filepath.Join(root.stateRoot, "config.yaml"))
		legacyBackup := filepath.Join(root.installRoot, "backup.local.yaml")
		if os.IsNotExist(configErr) && windowsLegacyFileOwned(legacyBackup) {
			if err := copyFileIfMissing(legacyBackup, filepath.Join(root.stateRoot, "backup.local.yaml")); err != nil {
				return err
			}
		}
	}
	copyLegacyState := normalizeGOOS(goos) != "windows" || windowsLegacyStateOwned(root.installRoot)
	// Copy bundled cores before a custom profile directory can create a parent
	// inside chrome/ and make the non-overwriting directory copy skip the bundle.
	if err := copyDirIfMissing(filepath.Join(root.installRoot, "chrome"), filepath.Join(root.stateRoot, "chrome")); err != nil {
		return err
	}
	if normalizeGOOS(goos) == "windows" && copyLegacyState {
		// An upgrade may have legacy SQLite/profile files beside the executable.
		// Copy only into a missing destination; leave the original and any existing
		// per-user data untouched. The caller holds the existing single-instance lock.
		if err := copyLegacyConfiguredPaths(root, goos); err != nil {
			return err
		}
	}

	if copyLegacyState {
		if err := copyFileIfMissing(
			filepath.Join(root.installRoot, "config.yaml"),
			filepath.Join(root.stateRoot, "config.yaml"),
		); err != nil {
			return err
		}
		if err := copyFileIfMissing(
			filepath.Join(root.installRoot, "proxies.yaml"),
			filepath.Join(root.stateRoot, "proxies.yaml"),
		); err != nil {
			return err
		}
	} else if normalizeGOOS(goos) == "windows" {
		// A new Windows user gets publisher defaults, never another user's
		// mutable configuration left in a shared legacy installation.
		if err := copyFileIfMissing(filepath.Join(root.installRoot, "config.defaults.yaml"), filepath.Join(root.stateRoot, "config.yaml")); err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(root.stateRoot, "data"), 0755)
}

func detect(appRoot string) roots {
	return detectForOS(appRoot, goruntime.GOOS)
}

func detectForOS(appRoot, goos string) roots {
	normalized := normalizeRoot(appRoot)
	cacheKey := buildCacheKey(goos, normalized)
	if cached, ok := rootsCache.Load(cacheKey); ok {
		return cached.(roots)
	}

	root := roots{
		installRoot: normalized,
		stateRoot:   normalized,
	}
	if shouldDetachStateRoot(goos, normalized) {
		root.stateRoot = userStateRootForOS(goos, normalized)
		root.detached = root.stateRoot != "" && root.stateRoot != normalized
	}

	actual, _ := rootsCache.LoadOrStore(cacheKey, root)
	return actual.(roots)
}

func buildCacheKey(goos, root string) string {
	return normalizeGOOS(goos) + "\x00" + root
}

func normalizeGOOS(goos string) string {
	return strings.ToLower(strings.TrimSpace(goos))
}

func shouldDetachStateRoot(goos, installRoot string) bool {
	switch normalizeGOOS(goos) {
	case "windows":
		// Detect installed roots independently of elevation/write permissions.
		if windowsInstalledRoot(installRoot) {
			return true
		}
		for _, key := range []string{"ProgramW6432", "ProgramFiles", "ProgramFiles(x86)"} {
			base := strings.TrimSpace(os.Getenv(key))
			if base != "" && (strings.EqualFold(installRoot, filepath.Clean(base)) ||
				strings.HasPrefix(strings.ToLower(installRoot), strings.ToLower(filepath.Clean(base))+string(filepath.Separator))) {
				return true
			}
		}
		// An unregistered portable root may not exist yet (for example before a
		// native backup import creates it). Do not adopt an installed user's state
		// merely because a write probe cannot create a file in that absent root.
		return false
	case "linux":
		return !dirWritable(installRoot)
	case "darwin":
		return isMacAppBundleRoot(installRoot) || !dirWritable(installRoot)
	default:
		return false
	}
}

func normalizeRoot(appRoot string) string {
	root := strings.TrimSpace(appRoot)
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return filepath.Clean(root)
}

func userStateRootForOS(goos, fallback string) string {
	switch normalizeGOOS(goos) {
	case "windows":
		if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); filepath.IsAbs(base) {
			return filepath.Join(base, appStateDirName)
		}
		if home := configuredHomeDir(); filepath.IsAbs(home) {
			return filepath.Join(home, "AppData", "Local", appStateDirName)
		}
		return "" // User state must never silently move to a temporary directory.
	case "linux":
		if base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); base != "" {
			return filepath.Join(base, appStateDirName)
		}
		if home := configuredHomeDir(); home != "" {
			return filepath.Join(home, ".local", "share", appStateDirName)
		}
	case "darwin":
		if home := configuredHomeDir(); home != "" {
			return filepath.Join(home, "Library", "Application Support", appStateDirName)
		}
	}
	if tmp := strings.TrimSpace(os.TempDir()); tmp != "" {
		return filepath.Join(tmp, appStateDirName)
	}
	return fallback
}

func configuredHomeDir() string {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home
	}
	if home, err := os.UserHomeDir(); err == nil {
		return strings.TrimSpace(home)
	}
	return ""
}

func isMacAppBundleRoot(dir string) bool {
	clean := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(dir)), "/")
	lower := strings.ToLower(clean)
	return strings.HasSuffix(lower, ".app/contents/macos") || strings.HasSuffix(lower, ".app/contents/resources")
}

func dirWritable(dir string) bool {
	file, err := os.CreateTemp(dir, ".ant-browser-write-test-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func useStateRoot(p string) bool {
	clean := filepath.ToSlash(fsutil.NormalizePathInput(p))
	if clean == "" || clean == "." {
		return false
	}
	return clean != "bin" && !strings.HasPrefix(clean, "bin/")
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("cannot copy non-regular runtime configuration: %s", src)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(dst), ".ant-browser-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	// Link publishes a complete file without replacing a concurrently created one.
	if err := os.Link(file.Name(), dst); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func copyDirIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(dst), ".ant-browser-copy-*")
	if err != nil {
		return err
	}
	// On error retain the partial copy for diagnosis, never at the final path.
	err = filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("cannot copy non-regular runtime data: %s", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := staging
		if rel != "." {
			target = filepath.Join(staging, rel)
		}
		if info.IsDir() {
			dirMode := info.Mode().Perm() | 0700
			return os.MkdirAll(target, dirMode)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
	if err != nil {
		return err
	}
	return os.Rename(staging, dst)
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
