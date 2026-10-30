package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/internal/kernelapi"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestKernelCopiesRequireFreshPublishedPreparedBinding(t *testing.T) {
	for _, kind := range []string{"default", "mode", "options"} {
		t.Run(kind, func(t *testing.T) {
			a, b, _, source, id := preparedKernelAppFixture(t)
			original, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "source", CoreId: id}, 7)
			if err != nil {
				t.Fatal(err)
			}
			copy := func() error {
				var err error
				switch kind {
				case "default":
					_, err = b.BrowserProfileCopy(original.ProfileId, "copy", 7)
				case "mode":
					_, err = b.BrowserProfileCopyWithMode(original.ProfileId, "copy", "regular", 7)
				default:
					_, err = b.BrowserProfileCopyWithOptions(original.ProfileId, "copy", BrowserProfileCopyOptions{Mode: "regular"}, 7)
				}
				return err
			}
			if err = copy(); err != nil {
				t.Fatal(err)
			}
			before := len(a.browserMgr.Profiles)
			source.err = kernelapi.ErrNotFound
			if err = copy(); !errors.Is(err, kernelapi.ErrNotFound) {
				t.Fatalf("copy rebound withdrawn release: %v", err)
			}
			if len(a.browserMgr.Profiles) != before || len(a.browserMgr.BrowserProcesses) != 0 {
				t.Fatal("rejected copy changed profiles or processes")
			}
			source.err = nil
			cores := a.kernels.ReadyCores()
			if len(cores) != 1 {
				t.Fatal("fixture missing prepared release")
			}
			executable, _, ok := browser.FindCoreExecutable(cores[0].CorePath)
			if !ok {
				t.Fatal("fixture executable missing")
			}
			if err = os.Remove(executable); err != nil {
				t.Fatal(err)
			}
			if err = copy(); err == nil {
				t.Fatal("copy admitted a missing prepared installation")
			}
			if len(a.browserMgr.Profiles) != before {
				t.Fatal("unprepared copy wrote profile")
			}
		})
	}
}

func TestLocalHTTPKernelMetadataReadCannotAdoptIdentityOrAPIRevocation(t *testing.T) {
	for _, kind := range []string{"identity", "api-grant"} {
		t.Run(kind, func(t *testing.T) {
			f, source, id := kernelHTTPFixture(t)
			source.artifactStarted = make(chan struct{})
			source.artifactRelease = make(chan struct{})
			body := fmt.Sprintf(`{"profile":{"profileName":"late","coreId":%q},"autoLaunch":false}`, id)
			result := make(chan int, 1)
			go func() { status, _ := f.request(t, http.MethodPost, "/api/profiles", body); result <- status }()
			select {
			case <-source.artifactStarted:
			case status := <-result:
				t.Fatalf("HTTP request skipped native metadata: %d", status)
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP metadata did not start")
			}
			if kind == "identity" {
				f.identity.replace()
			} else {
				if err := f.bound.AuthGrantAPI(7, false); err != nil {
					t.Fatal(err)
				}
			}
			close(source.artifactRelease)
			if status := <-result; status == http.StatusCreated {
				t.Fatal("stale HTTP metadata wrote profile")
			}
			f.assertProfiles(t, 0)
		})
	}
}

func TestKernelCopyBindsSourceAndIdentityAcrossMetadataRead(t *testing.T) {
	for _, kind := range []string{"source", "identity"} {
		t.Run(kind, func(t *testing.T) {
			a, b, auth, source, id := preparedKernelAppFixture(t)
			original, err := b.BrowserProfileCreate(BrowserProfileInput{ProfileName: "source", CoreId: id}, 7)
			if err != nil {
				t.Fatal(err)
			}
			source.artifactStarted = make(chan struct{})
			source.artifactRelease = make(chan struct{})
			result := make(chan error, 1)
			go func() { _, err := b.BrowserProfileCopy(original.ProfileId, "late-copy", 7); result <- err }()
			select {
			case <-source.artifactStarted:
			case err := <-result:
				t.Fatalf("copy skipped metadata admission: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("copy did not reach metadata")
			}
			if kind == "identity" {
				auth.replace()
			} else {
				_, err = a.browserMgr.UpdateChecked(original.ProfileId, BrowserProfileInput{ProfileName: "changed", CoreId: "competing-owner-binding"}, func() error { return nil })
				if err != nil {
					t.Fatal(err)
				}
			}
			close(source.artifactRelease)
			if err = <-result; err == nil {
				t.Fatal("stale copy committed")
			}
			if len(a.browserMgr.Profiles) != 1 {
				t.Fatal("stale copy wrote a new profile")
			}
		})
	}
}

func kernelHTTPFixture(t *testing.T) (*nativeIngressFixture, *kernelSourceFixture, string) {
	t.Helper()
	a, b, identity, source, id := preparedKernelAppFixture(t)
	service := launchcode.NewLaunchCodeService(launchcode.NewSQLiteLaunchCodeDAO(a.db.GetConn()))
	a.launchCodeSvc = service
	a.browserMgr.CodeProvider = service
	s := launchcode.NewLaunchServer(service, a, a.browserMgr, 0)
	s.SetIdentityAuthority(identity, a.scopedHTTP)
	s.SetAPIAuthConfig(launchcode.APIAuthConfig{Enabled: true, APIKey: t.Name()})
	a.launchServer = s
	if err := b.AuthGrantAPI(7, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})
	return &nativeIngressFixture{app: a, bound: b, identity: identity, server: s, client: client, base: fmt.Sprintf("http://127.0.0.1:%d", s.Port()), key: t.Name()}, source, id
}

func TestLocalHTTPKernelWritesUseNativeSelectionOwner(t *testing.T) {
	f, source, id := kernelHTTPFixture(t)
	body := fmt.Sprintf(`{"profile":{"profileName":"HTTP synthetic","coreId":%q},"autoLaunch":false}`, id)
	status, _ := f.request(t, http.MethodPost, "/api/profiles", body)
	if status != http.StatusCreated {
		t.Fatalf("published prepared HTTP creation failed: %d", status)
	}
	f.assertProfiles(t, 1)
	var original browser.Profile
	for _, p := range f.app.browserMgr.Profiles {
		original = *p
	}
	source.err = kernelapi.ErrNotFound
	status, _ = f.request(t, http.MethodPost, "/api/profiles", body)
	if status == http.StatusCreated {
		t.Fatal("HTTP fallback rebound withdrawn version")
	}
	f.assertProfiles(t, 1)
	// The original binding alone retains the withdrawal exception.
	status, _ = f.request(t, http.MethodPut, "/api/profiles/"+original.ProfileId, body)
	if status != http.StatusOK {
		t.Fatalf("HTTP update lost unchanged binding: %d", status)
	}
	source.err = nil
	source.artifact.ReleaseID = "20000000-0000-0000-0000-000000000002"
	unprepared := fmt.Sprintf(`{"profile":{"profileName":"not ready","coreId":%q},"autoLaunch":false}`, source.artifact.ReleaseID)
	status, _ = f.request(t, http.MethodPost, "/api/profiles", unprepared)
	if status == http.StatusCreated {
		t.Fatal("HTTP creation admitted unprepared version")
	}
	status, _ = f.request(t, http.MethodPut, "/api/profiles/"+original.ProfileId, unprepared)
	if status == http.StatusOK {
		t.Fatal("HTTP update admitted unprepared version")
	}
	f.assertProfiles(t, 1)
}
