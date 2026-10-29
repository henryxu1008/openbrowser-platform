package backend

import (
	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/proxy"
	"ant-chrome/internal/shuomiapi"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type storeViewContext struct {
	Context  shuomiapi.WorkbenchContext `json:"context"`
	Revision uint64                     `json:"revision"`
}

func storeJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", shuomiapi.ErrUnavailable
	}
	return string(data), nil
}
func (a *App) storeClient() (storeAPI, *browser.StoreDAO, error) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.store.client == nil {
		return nil, nil, shuomiapi.ErrConfiguration
	}
	if a.store.dao == nil {
		return nil, nil, browser.ErrStoreUnavailable
	}
	return a.store.client, a.store.dao, nil
}
func (a *App) storeCheck(revision uint64) error {
	if err := a.identity.Check(a.authGeneration); err != nil {
		return err
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.store.generation != a.authGeneration || a.store.revision != revision || a.store.switching || a.store.organization == "" {
		return shuomiapi.ErrContext
	}
	return nil
}
func (a *App) StoreWorkbenchContext() (string, error) {
	client, _, err := a.storeClient()
	if err != nil {
		return "", err
	}
	a.store.mu.Lock()
	if a.store.generation != a.authGeneration {
		a.store.generation = a.authGeneration
		a.store.revision++
		a.store.organization = ""
		a.store.switching = false
	}
	org, revision, pending := a.store.organization, a.store.revision, a.store.switching
	a.store.mu.Unlock()
	if pending {
		return "", shuomiapi.ErrContext
	}
	result, err := client.Context(a.ctx, a.authGeneration, org)
	if err != nil {
		return "", err
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.store.generation != a.authGeneration || a.store.revision != revision || a.store.switching {
		return "", shuomiapi.ErrContext
	}
	if result.EffectiveOrganizationID != nil && a.store.organization != *result.EffectiveOrganizationID {
		a.store.organization = *result.EffectiveOrganizationID
		a.store.revision++
	}
	return storeJSON(storeViewContext{Context: result, Revision: a.store.revision})
}
func (a *App) StoreWorkbenchSelectOrganization(org string, revision uint64) (string, error) {
	client, _, err := a.storeClient()
	if err != nil {
		return "", err
	}
	a.store.mu.Lock()
	if a.store.generation != a.authGeneration || a.store.revision != revision || a.store.switching {
		a.store.mu.Unlock()
		return "", shuomiapi.ErrContext
	}
	a.store.revision++
	next := a.store.revision
	a.store.organization = ""
	a.store.switching = true
	a.store.mu.Unlock()
	result, err := client.SelectOrganization(a.ctx, a.authGeneration, org)
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.store.generation != a.authGeneration || a.store.revision != next {
		return "", shuomiapi.ErrContext
	}
	a.store.switching = false
	if err != nil {
		return "", err
	}
	a.store.organization = org
	return storeJSON(storeViewContext{Context: result, Revision: next})
}
func (a *App) storeScope(storeID string, revision uint64) (storeAPI, *browser.StoreDAO, browser.StoreScope, error) {
	if err := a.storeCheck(revision); err != nil {
		return nil, nil, browser.StoreScope{}, err
	}
	client, dao, err := a.storeClient()
	if err != nil {
		return nil, nil, browser.StoreScope{}, err
	}
	identity, err := client.Identity(a.authGeneration)
	if err != nil {
		return nil, nil, browser.StoreScope{}, err
	}
	a.store.mu.Lock()
	org := a.store.organization
	a.store.mu.Unlock()
	return client, dao, browser.StoreScope{APIOrigin: client.Origin(), Issuer: identity.Issuer, Subject: identity.Subject, OrganizationID: org, StoreID: storeID}, nil
}
func (a *App) StoreWorkbenchList(search, platform string, page int, revision uint64) (string, error) {
	client, _, scope, err := a.storeScope("", revision)
	if err != nil {
		return "", err
	}
	result, err := client.Catalog(a.ctx, a.authGeneration, scope.OrganizationID, search, platform, page)
	if err != nil {
		return "", err
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	return storeJSON(result)
}
func (a *App) StoreWorkbenchGet(storeID string, revision uint64) (string, error) {
	client, _, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	result, err := client.Store(a.ctx, a.authGeneration, scope.OrganizationID, storeID)
	if err != nil {
		return "", err
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	return storeJSON(result)
}

type storeEnvironmentView struct {
	Binding       *browser.StoreBinding   `json:"binding"`
	Name          string                  `json:"name"`
	CoreID        string                  `json:"coreId"`
	ProxySummary  string                  `json:"proxySummary"`
	Running       bool                    `json:"running"`
	RuntimeKnown  bool                    `json:"runtimeKnown"`
	Pending       *browser.StoreOperation `json:"pending"`
	LastOperation *browser.StoreOperation `json:"lastOperation"`
	Reports       []browser.StoreReport   `json:"reports"`
	ObservedAt    time.Time               `json:"observedAt"`
}

func (a *App) storeEnvironmentState(scope browser.StoreScope, dao *browser.StoreDAO) (storeEnvironmentView, error) {
	result := storeEnvironmentView{ObservedAt: time.Now().UTC(), Reports: []browser.StoreReport{}}
	binding, found, err := dao.Binding(a.ctx, scope)
	if err != nil {
		return result, err
	}
	if found {
		result.Binding = &binding
		a.browserMgr.Mutex.Lock()
		p := a.browserMgr.Profiles[binding.ProfileID]
		if p == nil {
			a.browserMgr.Mutex.Unlock()
			return result, browser.ErrStoreUnavailable
		}
		if p != nil {
			result.Name = p.ProfileName
			result.CoreID = p.CoreId
			result.Running = p.Running
			result.RuntimeKnown = p.Running && a.browserMgr.BrowserProcesses[binding.ProfileID] != nil
			result.ProxySummary = "直连"
			proxyConfig := strings.TrimSpace(p.ProxyConfig)
			// Explicit config takes precedence over the saved proxy ID, as it does at launch.
			if proxy.DetectProxyProtocol(proxyConfig) != "direct" || proxyConfig == "" && p.ProxyId != "" && p.ProxyId != "__direct__" {
				result.ProxySummary = "本机代理"
			}
		}
		a.browserMgr.Mutex.Unlock()
	}
	if result.Pending, err = dao.Pending(a.ctx, scope); err != nil {
		return result, err
	}
	if found && !result.Running && result.Pending == nil {
		lastOpen, e := dao.LatestOpen(a.ctx, scope)
		if e != nil {
			return result, e
		}
		if lastOpen == nil {
			result.RuntimeKnown = true
		} else {
			a.browserMgr.Mutex.Lock()
			p := a.browserMgr.Profiles[binding.ProfileID]
			dir := ""
			if p != nil {
				dir = a.browserMgr.ResolveUserDataDir(p)
			}
			a.browserMgr.Mutex.Unlock()
			if dir != "" {
				if processes, e := observeStoreDirectoryProcesses(dir); e == nil && len(processes) == 0 {
					if dao.ObserveRuntime(a.ctx, scope, lastOpen.OperationID, "no_directory_process_observed") != nil {
						return result, browser.ErrStoreUnavailable
					}
					result.RuntimeKnown = true
				}
			}
		}
	}
	if result.LastOperation, err = dao.Latest(a.ctx, scope); err != nil {
		return result, err
	}
	if result.LastOperation != nil {
		result.Reports, err = dao.Reports(a.ctx, scope, result.LastOperation.OperationID)
	}
	return result, err
}
func (a *App) StoreEnvironmentGet(storeID string, revision uint64) (string, error) {
	client, dao, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	if _, err = client.Store(a.ctx, a.authGeneration, scope.OrganizationID, storeID); err != nil {
		return "", err
	}
	result, err := a.storeEnvironmentState(scope, dao)
	if err != nil {
		return "", err
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	return storeJSON(result)
}
func (a *App) StoreEnvironmentOptions(storeID string, revision uint64) (string, error) {
	client, dao, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	if err = client.CanUse(a.ctx, a.authGeneration, scope.OrganizationID); err != nil {
		return "", err
	}
	if _, err = client.Store(a.ctx, a.authGeneration, scope.OrganizationID, storeID); err != nil {
		return "", err
	}
	input := BrowserProfileInput{FingerprintArgs: append([]string{}, a.config.Browser.DefaultFingerprintArgs...)}
	binding, found, err := dao.Binding(a.ctx, scope)
	if err != nil {
		return "", err
	}
	if found {
		a.browserMgr.Mutex.Lock()
		p := a.browserMgr.Profiles[binding.ProfileID]
		if p == nil {
			a.browserMgr.Mutex.Unlock()
			return "", browser.ErrStoreUnavailable
		}
		input.ProfileName = p.ProfileName
		input.CoreId = p.CoreId
		input.ProxyConfig = p.ProxyConfig
		input.MemoryLimitMB = p.MemoryLimitMB
		input.RestoreLastSession = p.RestoreLastSession
		input.FingerprintArgs = append([]string{}, p.FingerprintArgs...)
		a.browserMgr.Mutex.Unlock()
	}
	a.browserMgr.Mutex.Lock()
	cores := append([]BrowserCore{}, a.browserMgr.ListCores()...)
	a.browserMgr.Mutex.Unlock()
	for i := range cores {
		cores[i].CorePath = ""
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	return storeJSON(struct {
		Cores       []BrowserCore         `json:"cores"`
		Input       BrowserProfileInput   `json:"input"`
		Binding     *browser.StoreBinding `json:"binding"`
		ProxyPolicy string                `json:"proxyPolicy"`
	}{cores, input, func() *browser.StoreBinding {
		if found {
			return &binding
		}
		return nil
	}(), "direct_or_local_only"})
}
func (a *App) StoreOperationGet(storeID, key string, revision uint64) (string, error) {
	// Like the server's None receipt route, an original receipt must survive
	// loss of the current organization selection/grant. The renderer uses its
	// view ticket to discard late display; the native boundary pins identity.
	_ = revision
	client, dao, err := a.storeClient()
	if err != nil {
		return "", err
	}
	identity, err := client.Identity(a.authGeneration)
	if err != nil {
		return "", err
	}
	scope, err := dao.OriginalOperationScope(a.ctx, browser.StoreScope{APIOrigin: client.Origin(), Issuer: identity.Issuer, Subject: identity.Subject, StoreID: storeID}, key)
	if err != nil {
		return "", err
	}
	local, err := dao.Operation(a.ctx, scope, key)
	if err != nil {
		return "", err
	}
	remote, err := client.Receipt(a.ctx, a.authGeneration, scope.OrganizationID, storeID, key)
	if err != nil {
		return "", err
	}
	if err = a.identity.Check(a.authGeneration); err != nil {
		return "", err
	}
	return storeJSON(struct {
		Local  browser.StoreOperation `json:"local"`
		Remote shuomiapi.Receipt      `json:"remote"`
	}{local, remote})
}
func (a *App) storeActionReceiver(revision uint64, started time.Time) (*App, error) {
	if err := a.storeCheck(revision); err != nil {
		return nil, err
	}
	check := func() error {
		if err := a.storeCheck(revision); err != nil {
			return err
		}
		if time.Since(started) >= 15*time.Second {
			return errors.New("store_execution_window_expired")
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	return &App{appState: a.appState, ctx: automation.WithAdmission(a.ctx, check), admission: check, authGeneration: a.authGeneration}, nil
}
func (a *App) storeReport(client storeAPI, dao *browser.StoreDAO, scope browser.StoreScope, key, kind, outcome, reason string) bool {
	report := browser.NewStoreReport(kind, outcome, reason)
	ctx := context.Background()
	if dao.SaveReport(ctx, scope, key, report) != nil {
		return false
	}
	at, _ := time.Parse(time.RFC3339Nano, report.ObservedAt)
	if _, err := client.Report(a.ctx, a.authGeneration, scope.OrganizationID, scope.StoreID, key, shuomiapi.Report{SchemaVersion: "store-browser-v1", Kind: kind, Outcome: outcome, ReasonCode: reason, ObservedAt: at}); err != nil {
		return false
	}
	return dao.ConfirmReport(ctx, scope, key, kind) == nil
}
func parseStoreVersion(value string) (int64, error) {
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil || v < 0 || v == int64(^uint64(0)>>1) || strconv.FormatInt(v, 10) != value {
		return 0, browser.ErrStoreConflict
	}
	return v, nil
}

func (a *App) StoreAuditList(storeID, raw string, revision uint64) (string, error) {
	client, _, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	var q shuomiapi.AuditQuery
	if len(raw) > 16384 {
		return "", shuomiapi.ErrContext
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || d.Decode(new(any)) != io.EOF {
		return "", shuomiapi.ErrContext
	}
	result, err := client.Audit(a.ctx, a.authGeneration, scope.OrganizationID, storeID, q)
	if err != nil {
		return "", err
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	return storeJSON(result)
}

func decodeStoreInput(raw string) (BrowserProfileInput, error) {
	var input BrowserProfileInput
	if len(raw) > 16384 {
		return input, browser.ErrStoreUnavailable
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
		return input, browser.ErrStoreUnavailable
	}
	return input, nil
}

// storeAdmit persists intent before the single remote request. Neither a replay
// receipt nor a failed/late response can grant a second execution.
func (a *App) storeAdmit(client storeAPI, dao *browser.StoreDAO, scope browser.StoreScope, b browser.StoreBinding, action string, revision uint64) (*App, string, error) {
	key := uuid.NewString()
	if err := a.storeCheck(revision); err != nil {
		return nil, "", err
	}
	if err := dao.Prepare(a.ctx, scope, b, key, action); err != nil {
		return nil, "", err
	}
	started := time.Now()
	receipt, err := client.Admit(a.ctx, a.authGeneration, scope.OrganizationID, scope.StoreID, key, shuomiapi.Command{SchemaVersion: "store-browser-v1", Action: action, BindingID: b.BindingID, ProfileID: b.ProfileID, BindingVersion: strconv.FormatInt(b.Version, 10)})
	if err != nil && shuomiapi.IsRejected(err) {
		if dao.Finish(context.Background(), scope, key, "failed", "admission_rejected") != nil {
			return nil, key, storeUnknown(key)
		}
		return nil, key, err
	}
	if err != nil || !receipt.ExecutionAuthorized {
		_ = dao.Finish(context.Background(), scope, key, "unknown", "result_unknown")
		return nil, key, storeUnknown(key)
	}
	scoped, err := a.storeActionReceiver(revision, started)
	if err == nil {
		err = dao.Phase(a.ctx, scope, key, "admitted")
	}
	if err != nil {
		_ = dao.Finish(context.Background(), scope, key, "unknown", "result_unknown")
		return nil, key, storeUnknown(key)
	}
	base := scoped.admission
	if action == "configure_environment" {
		return scoped, key, nil
	} // binding CAS is inside the same SQLite transaction
	scoped.admission = func() error {
		if err := base(); err != nil {
			return err
		}
		current, found, err := dao.Binding(a.ctx, scope)
		if err != nil {
			return err
		}
		if found && current.BindingID == b.BindingID && current.ProfileID == b.ProfileID && current.Version == b.Version {
			return nil
		}
		return browser.ErrStoreConflict
	}
	scoped.ctx = automation.WithAdmission(a.ctx, scoped.admission)
	return scoped, key, nil
}

func (a *App) StoreEnvironmentSave(storeID, expectedVersion, raw string, revision uint64) (string, error) {
	if !a.storeGeneralMu.TryLock() {
		return "", errors.New("store_workspace_busy")
	}
	defer a.storeGeneralMu.Unlock()
	client, dao, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	expected, err := parseStoreVersion(expectedVersion)
	if err != nil {
		return "", err
	}
	input, err := decodeStoreInput(raw)
	if err != nil {
		return "", err
	}
	b, found, err := dao.Binding(a.ctx, scope)
	if err != nil {
		return "", err
	}
	if found && b.Version != expected || !found && expected != 0 {
		return "", browser.ErrStoreConflict
	}
	if !found {
		b = browser.StoreBinding{BindingID: uuid.NewString(), ProfileID: uuid.NewString()}
	} else {
		state, e := a.storeEnvironmentState(scope, dao)
		if e != nil {
			return "", e
		}
		if state.Pending != nil || state.Running || !state.RuntimeKnown {
			return "", browser.ErrStoreUnknown
		}
	}
	b.Version = expected + 1
	b.BindingVersion = strconv.FormatInt(b.Version, 10)
	p, err := a.browserMgr.BuildStoreProfile(input, b)
	if err != nil {
		return "", err
	}
	a.browserMgr.Mutex.Lock()
	err = a.validateStoreLaunchProfile(p)
	a.browserMgr.Mutex.Unlock()
	if err != nil {
		return "", err
	}
	// Static readiness only; the existing kernel installer remains the owner.
	a.browserMgr.Mutex.Lock()
	core, ok := a.browserMgr.GetCore(p.CoreId)
	a.browserMgr.Mutex.Unlock()
	if !ok {
		return "", errors.New("store_kernel_required")
	}
	if _, _, ok = browser.FindCoreExecutable(a.browserMgr.ResolveCoreDirectory(core.CorePath)); !ok {
		return "", errors.New("store_kernel_required")
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	scoped, key, err := a.storeAdmit(client, dao, scope, b, "configure_environment", revision)
	if err != nil {
		return "", err
	}
	err = a.browserMgr.SaveStoreProfile(a.ctx, dao, scope, b, expected, key, p, scoped.admission)
	if err != nil {
		_ = dao.Finish(context.Background(), scope, key, "unknown", "result_unknown")
		a.storeReport(client, dao, scope, key, "configure_result", "unknown", "result_unknown")
		return "", storeUnknown(key)
	}
	a.storeReport(client, dao, scope, key, "configure_result", "succeeded", "configuration_saved")
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	result, err := a.storeEnvironmentState(scope, dao)
	if err != nil {
		return "", err
	}
	return storeJSON(result)
}

func (a *App) StoreEnvironmentOpen(storeID, expectedVersion string, revision uint64) (string, error) {
	if !a.storeGeneralMu.TryLock() {
		return "", errors.New("store_workspace_busy")
	}
	defer a.storeGeneralMu.Unlock()
	client, dao, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	expected, err := parseStoreVersion(expectedVersion)
	if err != nil {
		return "", err
	}
	b, found, err := dao.Binding(a.ctx, scope)
	if err != nil {
		return "", err
	}
	if !found || b.Version != expected {
		return "", browser.ErrStoreConflict
	}
	if _, err = client.Store(a.ctx, a.authGeneration, scope.OrganizationID, storeID); err != nil {
		return "", err
	}
	if err = client.CanUse(a.ctx, a.authGeneration, scope.OrganizationID); err != nil {
		return "", err
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	state, err := a.storeEnvironmentState(scope, dao)
	if err != nil {
		return "", err
	}
	if state.Pending != nil {
		return "", storeUnknown(state.Pending.OperationID)
	}
	if !state.RuntimeKnown {
		if open, e := dao.LastOpen(a.ctx, scope); e == nil {
			return "", storeUnknown(open.OperationID)
		}
		return "", browser.ErrStoreUnknown
	}
	if state.Running {
		if !state.RuntimeKnown {
			return "", browser.ErrStoreUnknown
		}
		if err = a.storeCheck(revision); err != nil {
			return "", err
		}
		return storeJSON(state)
	}
	scoped, key, err := a.storeAdmit(client, dao, scope, b, "open_environment", revision)
	if err != nil {
		return "", err
	}
	// Persist before original proxy preparation, filesystem or cmd.Start.
	if err = dao.Phase(a.ctx, scope, key, "attempted"); err != nil {
		return "", storeUnknown(key)
	}
	scoped.storeProfileID = b.ProfileID
	_, err = startManagedStoreBrowser(scoped, b.ProfileID)
	outcome, reason := "succeeded", "process_started"
	if err != nil {
		outcome, reason = "unknown", "result_unknown"
	}
	if dao.Finish(context.Background(), scope, key, outcome, reason) != nil {
		return "", storeUnknown(key)
	}
	a.storeReport(client, dao, scope, key, "open_result", outcome, reason)
	if err != nil {
		return "", storeUnknown(key)
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	result, err := a.storeEnvironmentState(scope, dao)
	if err != nil {
		return "", err
	}
	return storeJSON(result)
}

// Same original start chain; private seam permits deterministic offline tests.
var startManagedStoreBrowser = func(a *App, id string) (*BrowserProfile, error) { return a.BrowserInstanceStart(id) }

func storeUnknown(key string) error {
	return fmt.Errorf("%w: operationId=%s", browser.ErrStoreUnknown, key)
}

func (a *App) StoreEnvironmentClose(storeID string, revision uint64) (string, error) {
	client, dao, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return "", err
	}
	b, found, err := dao.Binding(a.ctx, scope)
	if err != nil {
		return "", err
	}
	if !found {
		return "", browser.ErrStoreUnavailable
	}
	open, err := dao.LastOpen(a.ctx, scope)
	if err != nil {
		return "", err
	}
	// Closing is cleanup of this host's admitted process, never a fresh command
	// selected by PID/path or an old disk-only runtime.
	err = NewAuthBoundApp(a).CloseOwnedBrowser(b.ProfileID)
	if err == nil {
		a.ownedBrowsers.mu.Lock()
		receipt := a.ownedBrowsers.items[b.ProfileID]
		a.ownedBrowsers.mu.Unlock()
		if receipt.monitor == nil {
			err = ownedCleanupUnknown("exit_monitor_missing")
		} else {
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			select {
			case <-receipt.monitor.Done():
			case <-timer.C:
				err = ownedCleanupUnknown("process_exit_unconfirmed")
			}
		}
	}
	outcome, reason := "succeeded", "owned_close_confirmed"
	if err != nil {
		outcome, reason = "unknown", "result_unknown"
	}
	a.storeReport(client, dao, scope, open.OperationID, "close_result", outcome, reason)
	if err != nil {
		return "", err
	}
	if err = a.storeCheck(revision); err != nil {
		return "", err
	}
	return storeJSON(struct {
		OperationID string    `json:"operationId"`
		Outcome     string    `json:"outcome"`
		ObservedAt  time.Time `json:"observedAt"`
	}{open.OperationID, outcome, time.Now().UTC()})
}

// Reuse the existing directory process parser. Unsupported/failed observation
// cannot be mistaken for an empty process list. No termination or restart.
var observeStoreDirectoryProcesses = func(dir string) ([]browserUserDataProcess, error) {
	if runtime.GOOS != "windows" {
		return nil, browser.ErrStoreUnknown
	}
	return findBrowserUserDataProcesses(dir)
}
