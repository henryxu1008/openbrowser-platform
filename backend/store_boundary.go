package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/proxy"
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/shuomiapi"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var errStoreBoundary = errors.New("store_managed_data_requires_scoped_operation")

type storeWorkspaceState struct {
	mu           sync.Mutex
	client       storeAPI
	dao          *browser.StoreDAO
	generation   uint64
	revision     uint64
	organization string
	switching    bool
}

// This port is private to the native Store use cases. Its production adapter is
// origin-bound; no renderer or runtime configuration can replace it.
type storeAPI interface {
	Origin() string
	Identity(uint64) (desktopauth.Identity, error)
	Context(context.Context, uint64, string) (shuomiapi.WorkbenchContext, error)
	SelectOrganization(context.Context, uint64, string) (shuomiapi.WorkbenchContext, error)
	CanUse(context.Context, uint64, string) error
	Catalog(context.Context, uint64, string, string, string, int) (shuomiapi.Catalog, error)
	Store(context.Context, uint64, string, string) (shuomiapi.Store, error)
	Admit(context.Context, uint64, string, string, string, shuomiapi.Command) (shuomiapi.Receipt, error)
	Receipt(context.Context, uint64, string, string, string) (shuomiapi.Receipt, error)
	Report(context.Context, uint64, string, string, string, shuomiapi.Report) (shuomiapi.Receipt, error)
	Audit(context.Context, uint64, string, string, shuomiapi.AuditQuery) (shuomiapi.AuditPage, error)
}

func (a *App) configureStoreClient(identityConfig desktopauth.Config) {
	// Basic Native identity alone does not provide the Store API resources.
	// Consume the same immutable publisher configuration as the manager.
	if identityConfig.APIProjectID == "" || len(identityConfig.ConfigurationIssues()) != 0 {
		return
	}
	config, err := shuomiapi.LoadConfig(a.appRoot)
	if err != nil {
		return
	}
	authority, ok := a.identity.(shuomiapi.Auth)
	if !ok {
		return
	}
	client, err := shuomiapi.New(config, authority)
	if err == nil {
		a.store.client = client
	}
}
func (a *App) hasStoreData() bool {
	if a == nil || a.appState == nil {
		return false
	}
	a.store.mu.Lock()
	d := a.store.dao
	a.store.mu.Unlock()
	return d != nil && d.HasManagedData()
}

// Existing generic entrypoints cannot prove the Store ownership of their
// outputs, config, filesystem, backups or background targets. Fail closed once
// managed evidence exists; store use cases and original owned cleanup have
// their own narrow authority. Only public kernel metadata remains generic.
func (a *App) guardStoreGeneral(method string) error {
	// Retired native capabilities must also reject crafted renderer calls.
	// NewApp always installs the platform owner, including when unconfigured.
	if a.kernels != nil && strings.HasPrefix(method, "BrowserCore") && method != "BrowserCoreList" {
		return errPlatformKernelOnly
	}
	if !a.hasStoreData() {
		return nil
	}
	switch method {
	case "BrowserCoreList":
		return nil
	}
	return errStoreBoundary
}

func (a *App) validateStoreLaunchProfile(p *BrowserProfile) error {
	if p == nil || p.ProfileId == "" || p.UserDataDir != p.ProfileId || len(p.LaunchArgs) > 0 || p.ProxyId != "" || len(p.FingerprintArgs) > 100 {
		return errStoreBoundary
	}
	for _, arg := range p.FingerprintArgs {
		key := browserFingerprintArgKey(arg)
		_, stable := fingerprintStableArgLabels[key]
		_, legacy := fingerprintNoEffectArgLabels[key]
		_, converted := fingerprintChrome144ConvertedArgLabels[key]
		if !stable && !legacy && !converted || strings.ContainsAny(arg, "\r\n\x00") {
			return errStoreBoundary
		}
	}
	if ok, _ := proxy.ValidateProxyConfig(p.ProxyConfig, nil, ""); !ok {
		return errStoreBoundary
	}
	// Canonicalize existing ancestors with the same OS path semantics. A UUID
	// directory may never be a junction/symlink into another profile.
	dir := filepath.Clean(a.browserMgr.ResolveUserDataDir(p))
	if filepath.Base(dir) != p.ProfileId {
		return errStoreBoundary
	}
	if info, err := os.Lstat(dir); err == nil {
		resolved, e := filepath.EvalSymlinks(dir)
		parent, pe := filepath.EvalSymlinks(filepath.Dir(dir))
		if e != nil || pe != nil || !info.IsDir() || !strings.EqualFold(filepath.Clean(resolved), filepath.Join(parent, p.ProfileId)) {
			return errStoreBoundary
		}
	} else if !os.IsNotExist(err) {
		return errStoreBoundary
	}
	for id, other := range a.browserMgr.Profiles {
		if id == p.ProfileId {
			continue
		}
		old := filepath.Clean(a.browserMgr.ResolveUserDataDir(other))
		if strings.EqualFold(old, dir) {
			return errStoreBoundary
		}
		if resolved, e := filepath.EvalSymlinks(old); e == nil {
			if target, e := filepath.EvalSymlinks(dir); e == nil && strings.EqualFold(resolved, target) {
				return errStoreBoundary
			}
		}
	}
	return nil
}

type storeGuardedAPI struct{ app *App }

func (g storeGuardedAPI) BeginOperation() func() {
	g.app.storeGeneralMu.RLock()
	return g.app.storeGeneralMu.RUnlock
}

func (g storeGuardedAPI) Check(generation uint64) error {
	if err := g.app.identity.Check(generation); err != nil {
		return err
	}
	return g.app.guardStoreGeneral("")
}
func (g storeGuardedAPI) CheckAPI() (uint64, error) {
	v, err := g.app.identity.CheckAPI()
	if err != nil {
		return 0, err
	}
	if err = g.app.guardStoreGeneral(""); err != nil {
		return 0, err
	}
	return v, nil
}
