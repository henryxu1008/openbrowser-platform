package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/kernelcache"
	"ant-chrome/internal/apiconfig"
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/kernelapi"
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"
)

var errPlatformKernelOnly = browser.ErrPublishedCoreSelection

func (a *App) configureKernelClient(identityConfig desktopauth.Config) {
	authority, _ := a.identity.(kernelcache.Authority)
	var source kernelcache.Source
	if identityConfig.APIProjectID != "" && len(identityConfig.ConfigurationIssues()) == 0 {
		if cfg, err := apiconfig.Load(a.appRoot); err == nil {
			if auth, ok := a.identity.(kernelapi.Auth); ok {
				client, err := kernelapi.New(cfg, auth, a.version)
				if err == nil {
					source = client
				}
			}
		}
	}
	// A dedicated cache is the only local installation fact. No CoreDAO copy,
	// arbitrary path adoption or existing profile migration is performed.
	a.kernels = kernelcache.New(filepath.Join(apppath.StateRoot(a.appRoot), "platform-kernels"), a.version, source, authority, func() (*http.Client, error) {
		client, _, err := a.unifiedProxyCoreHTTPClient(15*time.Minute, "__direct__")
		return client, err
	})
}

// Kernel methods have identity authority, independent of Store grants/data.
// They never hold storeGeneralMu during a potentially long preparation.
func (a *AuthBoundApp) scopedKernel(generation uint64) (*App, error) {
	if err := a.admit(generation); err != nil {
		return nil, err
	}
	return a.core.scoped(generation)
}

func (a *App) BrowserKernelCatalog() (string, error) {
	if a.kernels == nil {
		return "", kernelapi.ErrConfiguration
	}
	versions, err := a.kernels.Catalog(a.ctx, a.authGeneration)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(versions)
	return string(data), err
}

// Only an issued business ID crosses the renderer/native command boundary.
// Transport URLs, origins, credentials and filesystem paths remain native.
func (a *App) BrowserKernelPrepare(releaseID string) error {
	if a.kernels == nil {
		return kernelapi.ErrConfiguration
	}
	_, err := a.kernels.Prepare(a.ctx, a.authGeneration, releaseID, func(phase string, percent int) {
		a.emitRuntimeEvent("browser.kernel.progress", map[string]any{"releaseId": releaseID, "generation": a.authGeneration, "phase": phase, "percent": percent})
	})
	return err
}

func (a *App) requireKernelSelection(id, previous string) error {
	if a.kernels == nil {
		return nil
	} // Legacy package fixtures only.
	if id == "" {
		return errPlatformKernelOnly
	}
	return a.kernels.RequireSelection(a.ctx, a.authGeneration, id, id == previous)
}

// All copy consumers, including automation templates, create a new binding.
// Fetch publication outside the manager lock, then pin the source at commit.
func (a *App) copyBrowserProfile(profileID, name string, options *BrowserProfileCopyOptions) (*BrowserProfile, error) {
	check := a.captureAdmission()
	if err := check(); err != nil {
		return nil, err
	}
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	source := a.browserMgr.Profiles[profileID]
	coreID := ""
	if source != nil {
		coreID = source.CoreId
	}
	a.browserMgr.Mutex.Unlock()
	if source == nil {
		return nil, errPlatformKernelOnly
	}
	if err := a.requireKernelSelection(coreID, ""); err != nil {
		return nil, err
	}
	commitCheck := func(source *browser.Profile) error {
		if err := check(); err != nil {
			return err
		}
		if source.CoreId != coreID {
			return errPlatformKernelOnly
		}
		return nil
	}
	if options == nil {
		return a.browserMgr.CopyChecked(profileID, name, commitCheck)
	}
	return a.browserMgr.CopyWithOptionsChecked(profileID, name, *options, commitCheck)
}
