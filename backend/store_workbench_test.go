package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/shuomiapi"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"testing"
)

type storeAPIFixture struct {
	storeAPI
	auth           *mockSession
	admitted       int
	reports        int
	authorize      bool
	afterAdmission func()
	denied         error
}

func (s *storeAPIFixture) Origin() string { return "https://stores.example.test" }
func (s *storeAPIFixture) Identity(g uint64) (desktopauth.Identity, error) {
	v := s.auth.Status()
	return desktopauth.Identity{Issuer: v.Issuer, Subject: v.Subject}, s.auth.Check(g)
}
func (s *storeAPIFixture) Admit(_ context.Context, _ uint64, _, _, _ string, _ shuomiapi.Command) (shuomiapi.Receipt, error) {
	s.admitted++
	if s.afterAdmission != nil {
		s.afterAdmission()
	}
	return shuomiapi.Receipt{ExecutionAuthorized: s.authorize}, s.denied
}
func (s *storeAPIFixture) Store(context.Context, uint64, string, string) (shuomiapi.Store, error) {
	return shuomiapi.Store{}, s.denied
}
func (s *storeAPIFixture) CanUse(context.Context, uint64, string) error { return s.denied }
func (s *storeAPIFixture) Report(_ context.Context, _ uint64, _, _, _ string, r shuomiapi.Report) (shuomiapi.Receipt, error) {
	s.reports++
	return shuomiapi.Receipt{}, nil
}
func storeAppFixture(t *testing.T) (*App, *storeAPIFixture, browser.StoreScope) {
	t.Helper()
	root := t.TempDir()
	db, err := database.NewDB(filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao, err := browser.NewStoreDAO(db.GetConn())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	m := browser.NewManager(cfg, root)
	m.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())
	m.InitData()
	auth := newMockSession()
	api := &storeAPIFixture{auth: auth, authorize: true}
	a := &App{appState: &appState{identity: auth, config: cfg, db: db, browserMgr: m, businessReady: true}, ctx: context.Background(), authGeneration: 7}
	a.store.client = api
	a.store.dao = dao
	a.store.organization = "org-a"
	a.store.generation = 7
	a.store.revision = 1
	s := browser.StoreScope{APIOrigin: api.Origin(), Issuer: auth.Status().Issuer, Subject: auth.Status().Subject, OrganizationID: "org-a", StoreID: uuid.NewString()}
	return a, api, s
}
func installStoreFixture(t *testing.T, a *App, s browser.StoreScope) browser.StoreBinding {
	t.Helper()
	b := browser.StoreBinding{BindingID: uuid.NewString(), ProfileID: uuid.NewString(), Version: 1, BindingVersion: "1"}
	key := uuid.NewString()
	d := a.store.dao
	if err := d.Prepare(a.ctx, s, b, key, "configure_environment"); err != nil {
		t.Fatal(err)
	}
	if err := d.Phase(a.ctx, s, key, "admitted"); err != nil {
		t.Fatal(err)
	}
	p, err := a.browserMgr.BuildStoreProfile(browser.ProfileInput{ProfileName: "Shop"}, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.browserMgr.SaveStoreProfile(a.ctx, d, s, b, 0, key, p, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	return b
}
func TestStoreOpenLateAdmissionAndReplayNeverReachRuntime(t *testing.T) {
	for _, scenario := range []string{"replay", "switch", "logout"} {
		t.Run(scenario, func(t *testing.T) {
			a, api, s := storeAppFixture(t)
			installStoreFixture(t, a, s)
			old := startManagedStoreBrowser
			t.Cleanup(func() { startManagedStoreBrowser = old })
			calls := 0
			startManagedStoreBrowser = func(*App, string) (*BrowserProfile, error) { calls++; return nil, errors.New("must not run") }
			switch scenario {
			case "replay":
				api.authorize = false
			case "switch":
				api.afterAdmission = func() { a.store.mu.Lock(); a.store.revision++; a.store.organization = "org-b"; a.store.mu.Unlock() }
			case "logout":
				api.afterAdmission = func() { api.auth.SignOut(7) }
			}
			_, err := a.StoreEnvironmentOpen(s.StoreID, "1", 1)
			if !errors.Is(err, browser.ErrStoreUnknown) || calls != 0 || api.admitted != 1 {
				t.Fatalf("admitted runtime: %d %d %v", calls, api.admitted, err)
			}
			pending, e := a.store.dao.Pending(a.ctx, s)
			if e != nil || pending == nil {
				t.Fatal("lost unknown evidence")
			}
		})
	}
}
func TestStoreOpenAttemptIsDurableAndUnknownCannotUseNewKey(t *testing.T) {
	a, api, s := storeAppFixture(t)
	installStoreFixture(t, a, s)
	old := startManagedStoreBrowser
	t.Cleanup(func() { startManagedStoreBrowser = old })
	calls := 0
	startManagedStoreBrowser = func(scoped *App, id string) (*BrowserProfile, error) {
		calls++
		pending, err := a.store.dao.Pending(a.ctx, s)
		if err != nil || pending == nil || !pending.Attempted {
			t.Fatal("runtime before durable attempt")
		}
		if scoped.storeProfileID != id || scoped.captureAdmission()() != nil {
			t.Fatal("lost managed scope")
		}
		return nil, errors.New("synthetic uncertain start")
	}
	for range 2 {
		if _, err := a.StoreEnvironmentOpen(s.StoreID, "1", 1); !errors.Is(err, browser.ErrStoreUnknown) {
			t.Fatal(err)
		}
	}
	if calls != 1 || api.admitted != 1 || api.reports != 1 {
		t.Fatalf("automatic retry: %d %d %d", calls, api.admitted, api.reports)
	}
}
func TestStoreSaveUsesOriginalTransactionWithoutSecondSQLiteConnection(t *testing.T) {
	a, api, s := storeAppFixture(t)
	exe := filepath.Join(t.TempDir(), browser.CoreExecutableCandidates()[0])
	if err := os.WriteFile(exe, []byte("non-executable offline fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	a.config.Browser.Cores = []browser.Core{{CoreId: "fixture-core", CoreName: "Offline", CorePath: filepath.Dir(exe), IsDefault: true}}
	input, _ := json.Marshal(BrowserProfileInput{ProfileName: "Store", CoreId: "fixture-core"})
	if _, err := a.StoreEnvironmentSave(s.StoreID, "0", string(input), 1); err != nil {
		t.Fatal(err)
	}
	b, found, err := a.store.dao.Binding(a.ctx, s)
	if err != nil || !found || a.browserMgr.Profiles[b.ProfileID] == nil || api.admitted != 1 {
		t.Fatal("not atomically published")
	}
}
func TestStoreManagedBoundaryCoversOriginalFacadeAndRejectsDirectoryAlias(t *testing.T) {
	a, _, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	facade := NewAuthBoundApp(a)
	if _, err := facade.BrowserProfileList(7); !errors.Is(err, errStoreBoundary) {
		t.Fatalf("raw profile facade: %v", err)
	}
	if _, err := a.scopedHTTP(7); err == nil {
		t.Fatal("raw HTTP permitted")
	}
	if a.captureAdmission()() == nil {
		t.Fatal("background authority bypass")
	}
	p := a.browserMgr.Profiles[b.ProfileID]
	a.browserMgr.Profiles["old"] = &browser.Profile{ProfileId: "old", UserDataDir: p.UserDataDir}
	if a.validateStoreLaunchProfile(p) == nil {
		t.Fatal("legacy directory alias accepted")
	}
	delete(a.browserMgr.Profiles, "old")
	p.FingerprintArgs = []string{"--user-data-dir=other"}
	if a.validateStoreLaunchProfile(p) == nil {
		t.Fatal("escaped fingerprint accepted")
	}
}

func TestStoreKnownAdmissionRejectionRetainsFailureAndPermitsNewUserAction(t *testing.T) {
	a, api, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	api.denied = &shuomiapi.Rejection{Code: "PERMISSION_DENIED"}
	_, key, err := a.storeAdmit(api, a.store.dao, s, b, "open_environment", 1)
	if !shuomiapi.IsRejected(err) {
		t.Fatal(err)
	}
	pending, e := a.store.dao.Pending(a.ctx, s)
	if e != nil || pending != nil {
		t.Fatal("definite rejection locked future action")
	}
	old, e := a.store.dao.Operation(a.ctx, s, key)
	if e != nil || old.Phase != "failed" || old.Attempted {
		t.Fatal("rejection evidence wrong")
	}
	api.denied = nil
	if _, next, err := a.storeAdmit(api, a.store.dao, s, b, "open_environment", 1); err != nil || next == key || api.admitted != 2 {
		t.Fatal("restored grant cannot admit a new user action")
	}
}

func TestStoreRunningBranchStillRequiresRemoteAuthorization(t *testing.T) {
	a, api, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	a.browserMgr.Profiles[b.ProfileID].Running = true
	api.denied = &shuomiapi.Rejection{Code: "PERMISSION_DENIED"}
	value, err := a.StoreEnvironmentOpen(s.StoreID, "1", 1)
	if !shuomiapi.IsRejected(err) || value != "" || api.admitted != 0 {
		t.Fatal("revoked runtime read leaked")
	}
}

func TestStoreColdRuntimeRequiresCompleteDirectoryObservation(t *testing.T) {
	a, _, s := storeAppFixture(t)
	b := installStoreFixture(t, a, s)
	d := a.store.dao
	key := uuid.NewString()
	if err := d.Prepare(a.ctx, s, b, key, "open_environment"); err != nil {
		t.Fatal(err)
	}
	d.Phase(a.ctx, s, key, "admitted")
	d.Phase(a.ctx, s, key, "attempted")
	d.Finish(a.ctx, s, key, "succeeded", "process_started")
	old := observeStoreDirectoryProcesses
	t.Cleanup(func() { observeStoreDirectoryProcesses = old })
	for _, tc := range []struct {
		processes []browserUserDataProcess
		err       error
		known     bool
	}{{nil, errors.New("partial enumeration"), false}, {[]browserUserDataProcess{{PID: 123, DebugPort: 0}}, nil, false}, {[]browserUserDataProcess{}, nil, true}} {
		observeStoreDirectoryProcesses = func(string) ([]browserUserDataProcess, error) { return tc.processes, tc.err }
		state, err := a.storeEnvironmentState(s, d)
		if err != nil || state.RuntimeKnown != tc.known {
			t.Fatalf("cold proof: %+v %v", state, err)
		}
	}
	operation, err := d.Operation(a.ctx, s, key)
	if err != nil || operation.Outcome != "succeeded" || operation.RuntimeObservation != "no_directory_process_observed" {
		t.Fatal("observation overwrote original fact or was not retained")
	}
}
