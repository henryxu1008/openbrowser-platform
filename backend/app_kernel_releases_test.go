package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/kernelcache"
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/kernelapi"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type kernelSourceFixture struct {
	started, release                 chan struct{}
	artifactStarted, artifactRelease chan struct{}
	artifact                         kernelapi.Artifact
	err                              error
}

func (*kernelSourceFixture) Origin() string { return "https://api.example.test" }
func (s *kernelSourceFixture) Catalog(context.Context, uint64) ([]kernelapi.Release, error) {
	if s.started != nil {
		close(s.started)
		<-s.release
	}
	return []kernelapi.Release{{ReleaseID: "10000000-0000-0000-0000-000000000001", Version: "130.0.0.1", OS: runtime.GOOS, Arch: runtime.GOARCH, MinClientVersion: "1.0.0"}}, nil
}
func (s *kernelSourceFixture) Artifact(context.Context, uint64, string) (kernelapi.Artifact, error) {
	if s.artifactStarted != nil {
		close(s.artifactStarted)
		<-s.artifactRelease
	}
	if s.err != nil {
		return kernelapi.Artifact{}, s.err
	}
	if s.artifact.ReleaseID == "" {
		return kernelapi.Artifact{}, kernelapi.ErrNotFound
	}
	return s.artifact, nil
}

type kernelAuthorityFixture struct{ *mockSession }

func (a kernelAuthorityFixture) WithIdentity(g uint64, f func(desktopauth.Identity) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status.Generation != g || a.status.State != "signed_in" {
		return desktopauth.ErrUnauthenticated
	}
	return f(desktopauth.Identity{})
}

func TestPlatformKernelFacadeRejectsAllRetiredCapabilities(t *testing.T) {
	a, b, _ := authFixture(t)
	if a.kernels == nil {
		t.Fatal("production constructor omitted platform owner")
	}
	v := reflect.ValueOf(b)
	typ := v.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !strings.HasPrefix(method.Name, "BrowserCore") || method.Name == "BrowserCoreList" {
			continue
		}
		t.Run(method.Name, func(t *testing.T) {
			fn := v.Method(i)
			args := make([]reflect.Value, fn.Type().NumIn())
			for j := range args {
				args[j] = reflect.Zero(fn.Type().In(j))
			}
			args[len(args)-1] = reflect.ValueOf(uint64(7))
			out := fn.Call(args)
			err, ok := out[len(out)-1].Interface().(error)
			if !ok || !errors.Is(err, errPlatformKernelOnly) {
				t.Fatalf("legacy native capability entered delegation: %v", out)
			}
		})
	}
	if _, err := b.BrowserKernelCatalog(7); !errors.Is(err, kernelapi.ErrConfiguration) {
		t.Fatalf("missing platform config fell back: %v", err)
	}
}

func TestPlatformKernelCatalogIsStoreIndependentAndDoesNotHoldGeneralLock(t *testing.T) {
	a, api, scope := storeAppFixture(t)
	installStoreFixture(t, a, scope)
	if !a.hasStoreData() {
		t.Fatal("fixture has no managed evidence")
	}
	s := &kernelSourceFixture{started: make(chan struct{}), release: make(chan struct{})}
	a.kernels = kernelcache.New(t.TempDir(), "1.8.0", s, kernelAuthorityFixture{api.auth}, nil)
	b := NewAuthBoundApp(a)
	if _, err := b.BrowserProfileList(7); !errors.Is(err, errStoreBoundary) {
		t.Fatalf("generic Store boundary widened: %v", err)
	}
	result := make(chan error, 1)
	go func() { _, err := b.BrowserKernelCatalog(7); result <- err }()
	select {
	case <-s.started:
	case err := <-result:
		t.Fatalf("catalog failed before source: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("catalog did not reach source")
	}
	if !a.storeGeneralMu.TryLock() {
		close(s.release)
		<-result
		t.Fatal("network read retained the general Store lock")
	}
	a.storeGeneralMu.Unlock()
	close(s.release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

type kernelTransportFixture func(*http.Request) (*http.Response, error)

func (f kernelTransportFixture) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func preparedKernelAppFixture(t *testing.T) (*App, *AuthBoundApp, *mockSession, *kernelSourceFixture, string) {
	t.Helper()
	a, b, auth := authFixture(t)
	a.ctx = context.Background()
	a.config = config.DefaultConfig()
	a.db = newProfilePackageDatabase(t, a.appRoot)
	a.browserMgr = browser.NewManager(a.config, a.appRoot)
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(a.db.GetConn())
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	f, err := w.Create(browser.CoreExecutableCandidates()[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("synthetic; never execute")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	payload := data.Bytes()
	digest := sha256.Sum256(payload)
	id := "10000000-0000-0000-0000-000000000001"
	s := &kernelSourceFixture{artifact: kernelapi.Artifact{Release: kernelapi.Release{ReleaseID: id, Version: "130.0.0.1", OS: runtime.GOOS, Arch: runtime.GOARCH, MinClientVersion: "1.0.0"}, ArchiveFormat: "zip", Size: int64(len(payload)), SHA256: hex.EncodeToString(digest[:]), DownloadURL: "https://cdn.example.test/core.zip", DownloadOrigins: []string{"https://cdn.example.test"}}}
	a.kernels = kernelcache.New(filepath.Join(a.appRoot, "kernels"), "1.8.0", s, kernelAuthorityFixture{auth}, func() (*http.Client, error) {
		return &http.Client{Transport: kernelTransportFixture(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: r}, nil
		})}, nil
	})
	a.browserMgr.PublishedCores = a.kernels.ReadyCores
	if _, err = a.kernels.Prepare(a.ctx, 7, id, nil); err != nil {
		t.Fatal(err)
	}
	return a, b, auth, s, id
}

func TestPreparedKernelNativeSaveWithdrawalAndReadBoundary(t *testing.T) {
	a, b, _, s, id := preparedKernelAppFixture(t)
	created, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "synthetic", CoreId: id}, 7)
	if err != nil {
		t.Fatal(err)
	}
	cores, err := b.BrowserCoreList(7)
	if err != nil || len(cores) != 1 || cores[0].CorePath != "" {
		t.Fatalf("renderer core view leaked path: %v / %v", cores, err)
	}
	s.err = kernelapi.ErrNotFound
	if _, err = b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "withdrawn", CoreId: id}, 7); !errors.Is(err, kernelapi.ErrNotFound) {
		t.Fatal("new binding bypassed withdrawal")
	}
	if _, err = b.BrowserProfileUpdate(created.ProfileId, BrowserProfileInput{ProfileName: "same binding", CoreId: id}, 7); err != nil {
		t.Fatal(err)
	}
	if len(a.browserMgr.Profiles) != 1 || len(a.browserMgr.BrowserProcesses) != 0 {
		t.Fatal("selection/save entered process lifecycle or duplicated profile")
	}
}

func TestKernelSaveRejectsIdentityOrBindingChangeDuringMetadataRead(t *testing.T) {
	for _, mode := range []string{"identity", "binding"} {
		t.Run(mode, func(t *testing.T) {
			a, b, auth, s, id := preparedKernelAppFixture(t)
			previous, err := a.browserMgr.Create(BrowserProfileInput{ProfileName: "before", CoreId: "previous-binding"})
			if err != nil {
				t.Fatal(err)
			}
			s.artifactStarted = make(chan struct{})
			s.artifactRelease = make(chan struct{})
			result := make(chan error, 1)
			go func() {
				_, err := b.BrowserProfileUpdate(previous.ProfileId, BrowserProfileInput{ProfileName: "late write", CoreId: id}, 7)
				result <- err
			}()
			select {
			case <-s.artifactStarted:
			case err := <-result:
				t.Fatalf("save failed before metadata read: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("save did not reach metadata source")
			}
			if mode == "identity" {
				auth.replace()
			} else {
				if _, err = a.browserMgr.Update(previous.ProfileId, BrowserProfileInput{ProfileName: "intervening", CoreId: "other-binding"}); err != nil {
					t.Fatal(err)
				}
			}
			close(s.artifactRelease)
			if err = <-result; err == nil {
				t.Fatal("stale metadata write committed")
			}
			a.browserMgr.Mutex.Lock()
			p := a.browserMgr.Profiles[previous.ProfileId]
			name := p.ProfileName
			a.browserMgr.Mutex.Unlock()
			if name == "late write" {
				t.Fatal("late profile changed persistence")
			}
		})
	}
}
