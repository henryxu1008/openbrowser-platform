package kernelcache

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/kernelapi"
)

type testAuthority struct {
	mu         sync.Mutex
	generation uint64
}

func (a *testAuthority) Check(g uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if g != a.generation {
		return desktopauth.ErrUnauthenticated
	}
	return nil
}
func (a *testAuthority) WithIdentity(g uint64, f func(desktopauth.Identity) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if g != a.generation {
		return desktopauth.ErrUnauthenticated
	}
	return f(desktopauth.Identity{})
}
func (a *testAuthority) revoke() { a.mu.Lock(); a.generation++; a.mu.Unlock() }

type testSource struct {
	artifact kernelapi.Artifact
	err      error
	calls    atomic.Int32
}

func (s *testSource) Origin() string { return "https://api.example.test" }
func (s *testSource) Catalog(context.Context, uint64) ([]kernelapi.Release, error) {
	return []kernelapi.Release{s.artifact.Release}, s.err
}
func (s *testSource) Artifact(context.Context, uint64, string) (kernelapi.Artifact, error) {
	s.calls.Add(1)
	return s.artifact, s.err
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixture(t *testing.T) (*Manager, *testSource, *testAuthority, *atomic.Int32, []byte) {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	name := browser.CoreExecutableCandidates()[0]
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("synthetic executable; never run")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	sum := sha256.Sum256(data)
	s := &testSource{artifact: kernelapi.Artifact{Release: kernelapi.Release{ReleaseID: "10000000-0000-0000-0000-000000000001", Version: "130.0.0.1", OS: runtime.GOOS, Arch: runtime.GOARCH, MinClientVersion: "1.0.0"}, ArchiveFormat: "zip", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), DownloadURL: "https://cdn.example.test/core.zip", DownloadOrigins: []string{"https://cdn.example.test"}}}
	a := &testAuthority{generation: 1}
	calls := &atomic.Int32{}
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential reached artifact transport")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Header: make(http.Header), Request: r}, nil
	})}
	m := New(filepath.Join(t.TempDir(), "kernels"), "1.8.0", s, a, func() (*http.Client, error) { return client, nil })
	return m, s, a, calls, data
}

func TestPrepareImmutableCacheRepairAndOriginBinding(t *testing.T) {
	m, s, _, calls, _ := fixture(t)
	core, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.ReadyCores()) != 1 || calls.Load() != 1 {
		t.Fatal("installation missing")
	}
	s.artifact.Recommended = true
	again, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil)
	if err != nil || again.CorePath != core.CorePath || calls.Load() != 1 {
		t.Fatalf("cache reuse: %v", err)
	}
	versions, err := m.Catalog(context.Background(), 1)
	if err != nil || len(versions) != 1 || !versions[0].Installed {
		t.Fatal("catalog did not join install fact")
	}
	data, err := os.ReadFile(filepath.Join(core.CorePath, receiptName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "cdn.example") || strings.Contains(string(data), "downloadUrl") {
		t.Fatal("transport metadata persisted")
	}
	other := New(m.root, m.version, s, m.authority, m.httpClient)
	other.origin = "https://other.example.test"
	if len(other.ReadyCores()) != 0 {
		t.Fatal("cross-publisher cache adoption")
	}
	exe, _, _ := browser.FindCoreExecutable(core.CorePath)
	if err = os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	if len(m.ReadyCores()) != 0 {
		t.Fatal("missing executable remained ready")
	}
	repaired, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil)
	if err != nil || repaired.CorePath == core.CorePath || calls.Load() != 2 {
		t.Fatalf("repair: %v", err)
	}
	if _, err = os.Stat(core.CorePath); err != nil {
		t.Fatal("repair removed old immutable directory")
	}
	s.artifact.SHA256 = strings.Repeat("a", 64)
	if _, err = m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil); !errors.Is(err, ErrIdentity) || calls.Load() != 2 {
		t.Fatalf("identity mutation accepted: %v", err)
	}
	s.err = kernelapi.ErrNotFound
	if _, err = m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil); !errors.Is(err, kernelapi.ErrNotFound) {
		t.Fatal("withdrawal bypassed by cache")
	}
	if len(m.ReadyCores()) != 1 {
		t.Fatal("withdrawal invalidated existing prepared binding")
	}
	if err = m.RequireSelection(context.Background(), 1, s.artifact.ReleaseID, true); err != nil {
		t.Fatalf("unchanged withdrawn binding: %v", err)
	}
	if err = m.RequireSelection(context.Background(), 1, s.artifact.ReleaseID, false); !errors.Is(err, kernelapi.ErrNotFound) {
		t.Fatal("new save selected withdrawn release")
	}
}

func TestPreparationFailureNeverPublishes(t *testing.T) {
	for _, kind := range []string{"hash", "size", "invalid-source", "logout-download", "logout-commit"} {
		t.Run(kind, func(t *testing.T) {
			m, s, a, calls, _ := fixture(t)
			var progress Progress
			switch kind {
			case "hash":
				s.artifact.SHA256 = strings.Repeat("a", 64)
			case "size":
				s.artifact.Size++
			case "invalid-source":
				s.artifact.DownloadURL = "http://cdn.example.test/core.zip"
			case "logout-download":
				progress = func(phase string, _ int) {
					if phase == "downloading" {
						a.revoke()
					}
				}
			case "logout-commit":
				progress = func(phase string, _ int) {
					if phase == "extracting" {
						a.revoke()
					}
				}
			}
			if _, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, progress); err == nil {
				t.Fatal("unexpected success")
			}
			if len(m.ReadyCores()) != 0 {
				t.Fatal("failed installation became ready")
			}
			if kind == "invalid-source" && calls.Load() != 0 {
				t.Fatal("invalid metadata reached network")
			}
			entries, _ := os.ReadDir(m.root)
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), ".prepare-") {
					t.Fatal("private stage leaked")
				}
			}
		})
	}
}

func TestPrepareSingleFlightAndGenerationBoundary(t *testing.T) {
	m, s, a, _, data := fixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	m.httpClient = func() (*http.Client, error) {
		return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			close(started)
			<-release
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Header: make(http.Header), Request: r}, nil
		})}, nil
	}
	err1 := make(chan error, 1)
	go func() { _, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil); err1 <- err }()
	<-started
	err2 := make(chan error, 1)
	go func() { _, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil); err2 <- err }()
	// The second artifact lookup happens before joining the immutable flight.
	for s.calls.Load() < 2 {
		runtime.Gosched()
	}
	a.revoke()
	if _, err := m.Prepare(context.Background(), 2, s.artifact.ReleaseID, nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("new identity joined old flight: %v", err)
	}
	close(release)
	if <-err1 == nil || <-err2 == nil || calls.Load() != 1 || len(m.ReadyCores()) != 0 {
		t.Fatal("flight crossed identity or duplicated download")
	}
}

func TestDownloadRedirectPolicy(t *testing.T) {
	for _, kind := range []string{"three", "four", "other-host", "downgrade", "query"} {
		t.Run(kind, func(t *testing.T) {
			m, s, _, _, data := fixture(t)
			var calls int
			m.httpClient = func() (*http.Client, error) {
				return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
						t.Fatal("credential leak")
					}
					h := make(http.Header)
					status := 200
					body := data
					hops := 3
					if kind == "four" {
						hops = 4
					}
					if calls <= hops {
						status = 302
						body = nil
						location := "https://cdn.example.test/next?signature=test"
						if kind == "other-host" {
							location = "https://evil.example.test/core"
						}
						if kind == "downgrade" {
							location = "http://cdn.example.test/core"
						}
						h.Set("Location", location)
					}
					return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: r}, nil
				})}, nil
			}
			_, err := m.Prepare(context.Background(), 1, s.artifact.ReleaseID, nil)
			if kind == "three" || kind == "query" {
				if err != nil || calls != 4 {
					t.Fatalf("approved redirects failed %v / %d", err, calls)
				}
			} else if err == nil {
				t.Fatal("unsafe redirect accepted")
			}
		})
	}
}
