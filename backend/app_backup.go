package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"strings"
	"time"
)

type BackupScope = backup.Scope
type BackupManifest = backup.Manifest

func (a *App) backupBuildScope() (backup.Scope, error) {
	cfg := a.config
	if cfg != nil {
		// Scope construction receives the state root. Resolve core origins with
		// the same installation-aware lookup as launches before passing a copy;
		// exporting must never rewrite the user's saved relative core settings.
		copyConfig := *cfg
		copyConfig.Browser.Cores = append([]config.BrowserCore(nil), cfg.Browser.Cores...)
		manager := browser.NewManager(cfg, a.appRoot)
		for i := range copyConfig.Browser.Cores {
			core := &copyConfig.Browser.Cores[i]
			if strings.TrimSpace(core.CorePath) == "" {
				continue
			}
			core.CorePath = manager.ResolveCoreDirectory(core.CorePath)
		}
		cfg = &copyConfig
	}
	return backup.BuildScope(backup.BuildOptions{
		AppRoot:      a.appStateRootAbs(),
		Config:       cfg,
		UserDataRoot: a.backupResolveUserDataRoot(a.config),
		DatabasePath: a.backupResolveDBPath(a.config),
	})
}

// BackupGetScopeDefinition 返回当前环境下的备份范围定义（第一阶段：范围与包格式）。
func (a *App) BackupGetScopeDefinition() (BackupScope, error) {
	return a.backupBuildScope()
}

// BackupGetManifestTemplate 返回 manifest 结构预览（不执行实际导出）。
func (a *App) BackupGetManifestTemplate() (BackupManifest, error) {
	scope, err := a.BackupGetScopeDefinition()
	if err != nil {
		return BackupManifest{}, err
	}
	return backup.BuildManifest(scope, a.appName(), a.appVersion(), time.Now()), nil
}
