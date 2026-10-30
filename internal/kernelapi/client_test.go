package kernelapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"ant-chrome/internal/apiconfig"
	"ant-chrome/internal/desktopauth"
)

const firstID = "10000000-0000-4000-8000-000000000001"
const secondID = "10000000-0000-4000-8000-00000000000b"

type fixtureAuth struct {
	revoked atomic.Bool
}

func (a *fixtureAuth) Check(g uint64) error {
	if a.revoked.Load() || g != 7 {
		return desktopauth.ErrUnauthenticated
	}
	return nil
}
func (a *fixtureAuth) AuthorizeRequest(g uint64, r *http.Request) error {
	if err := a.Check(g); err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer native-fixture-only")
	return nil
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func fixtureClient(t *testing.T, auth *fixtureAuth, handler roundTrip) *Client {
	t.Helper()
	c, err := newClient(apiconfig.Config{APIOrigin: "https://api.example.test"}, auth, "1.8.1", "windows", "amd64", &http.Client{Transport: handler})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func releaseFixture(id string) Release {
	return Release{ReleaseID: id, Version: "150.0.7871.186", OS: "windows", Arch: "amd64", MinClientVersion: "1.8.0", MaxClientVersionExclusive: "1.9.0", Recommended: true}
}
func pageFixture(releases []Release, next string) string {
	data, _ := json.Marshal(struct {
		Releases []Release `json:"releases"`
		Next     string    `json:"nextCursor"`
	}{releases, next})
	return string(data)
}
func artifactFixture() map[string]any {
	return map[string]any{"releaseId": secondID, "version": "150.0.7871.186", "os": "windows", "arch": "amd64", "minClientVersion": "1.8.0", "maxClientVersionExclusive": "1.9.0", "recommended": true, "archiveFormat": "zip", "size": 1024, "sha256": strings.Repeat("a", 64), "downloadUrl": "https://assets.example.test/v150/windows-amd64.zip"}
}

func TestCatalogContinuesEmptyCompatibilityPageAndPinsNativeQuery(t *testing.T) {
	a := &fixtureAuth{}
	calls := 0
	c := fixtureClient(t, a, func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Host != "api.example.test" || r.URL.Path != "/api/v1/browser/kernel-releases" || r.Header.Get("Authorization") != "Bearer native-fixture-only" || r.Header.Get("X-Requested-Organization-ID") != "" || q.Get("os") != "windows" || q.Get("arch") != "amd64" || q.Get("clientVersion") != "1.8.1" || q.Get("limit") != "100" || r.Body != nil {
			t.Fatal("catalog request crossed the platform/native authority boundary")
		}
		if calls == 1 {
			if len(q) != 4 || q.Get("cursor") != "" {
				t.Fatal("unexpected first-page query")
			}
			return response(r, 200, pageFixture([]Release{}, firstID)), nil
		}
		if len(q) != 5 || q.Get("cursor") != firstID || calls != 2 {
			t.Fatal("empty page stopped or pagination escaped its cursor")
		}
		return response(r, 200, pageFixture([]Release{releaseFixture(secondID)}, "")), nil
	})
	got, err := c.Catalog(context.Background(), 7)
	if err != nil || len(got) != 1 || got[0].ReleaseID != secondID || calls != 2 {
		t.Fatalf("catalog = %+v, calls=%d, error=%v", got, calls, err)
	}
}

func TestArtifactMetadataIsBoundAndNeverSerializedToRenderer(t *testing.T) {
	a := &fixtureAuth{}
	c := fixtureClient(t, a, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/browser/kernel-releases/"+secondID+"/artifact" || len(r.URL.Query()) != 3 || r.URL.Query().Get("clientVersion") != "1.8.1" {
			t.Fatal("artifact lookup did not bind the requested release and native compatibility")
		}
		data, _ := json.Marshal(artifactFixture())
		return response(r, 200, string(data)), nil
	})
	got, err := c.Artifact(context.Background(), 7, secondID)
	if err != nil || got.Size != 1024 || got.SHA256 != strings.Repeat("a", 64) || got.DownloadURL == "" {
		t.Fatalf("artifact metadata unavailable: %v", err)
	}
	public, err := json.Marshal(got)
	if err != nil || strings.Contains(string(public), "assets.example") || strings.Contains(string(public), "sha256") || strings.Contains(string(public), "downloadUrl") {
		t.Fatal("native artifact details crossed the renderer serialization boundary")
	}
}

func TestArtifactRejectsSubstitutedOrAmbiguousMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"wrong-release", func(m map[string]any) { m["releaseId"] = firstID }},
		{"wrong-os", func(m map[string]any) { m["os"] = "linux" }},
		{"wrong-arch", func(m map[string]any) { m["arch"] = "arm64" }},
		{"client-too-old", func(m map[string]any) { m["minClientVersion"] = "1.9.0" }},
		{"upper-exclusive", func(m map[string]any) { m["maxClientVersionExclusive"] = "1.8.1" }},
		{"missing-field", func(m map[string]any) { delete(m, "maxClientVersionExclusive") }},
		{"null-field", func(m map[string]any) { m["maxClientVersionExclusive"] = nil }},
		{"unknown-field", func(m map[string]any) { m["corePath"] = "C:/foreign" }},
		{"uppercase-hash", func(m map[string]any) { m["sha256"] = strings.Repeat("A", 64) }},
		{"invalid-hash", func(m map[string]any) { m["sha256"] = strings.Repeat("z", 64) }},
		{"empty-size", func(m map[string]any) { m["size"] = 0 }},
		{"oversized", func(m map[string]any) { m["size"] = MaxArtifactBytes + 1 }},
		{"unsupported-format", func(m map[string]any) { m["archiveFormat"] = "exe" }},
		{"http-asset", func(m map[string]any) { m["downloadUrl"] = "http://assets.example.test/v150.zip" }},
		{"asset-userinfo", func(m map[string]any) { m["downloadUrl"] = "https://user:fixture-secret@assets.example.test/v150.zip" }},
		{"asset-query", func(m map[string]any) { m["downloadUrl"] = "https://assets.example.test/v150.zip?secret=fixture" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := artifactFixture()
			tc.change(m)
			data, _ := json.Marshal(m)
			c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) { return response(r, 200, string(data)), nil })
			if _, err := c.Artifact(context.Background(), 7, secondID); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatalf("unsafe metadata accepted or exposed: %v", err)
			}
		})
	}
	for _, suffix := range []string{`,"size":1024}`, `} {}`} {
		m, _ := json.Marshal(artifactFixture())
		body := strings.TrimSuffix(string(m), "}") + suffix
		c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) { return response(r, 200, body), nil })
		if _, err := c.Artifact(context.Background(), 7, secondID); !errors.Is(err, ErrUnavailable) {
			t.Fatal("duplicate fields or trailing JSON accepted")
		}
	}
}

func TestCatalogRejectsInvalidPaginationAndCompatibility(t *testing.T) {
	wrongOS := releaseFixture(firstID)
	wrongOS.OS = "darwin"
	for _, body := range []string{
		pageFixture([]Release{releaseFixture(secondID), releaseFixture(firstID)}, ""),
		pageFixture([]Release{releaseFixture(firstID), releaseFixture(firstID)}, ""),
		pageFixture([]Release{wrongOS}, ""),
		pageFixture([]Release{releaseFixture(secondID)}, firstID),
		`{"releases":null,"nextCursor":""}`,
		`{"releases":[],"nextCursor":"../next"}`,
		`{"releases":[],"nextCursor":"","nextCursor":""}`,
		`{"releases":[],"nextCursor":"","extra":true}`,
	} {
		c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) { return response(r, 200, body), nil })
		if _, err := c.Catalog(context.Background(), 7); !errors.Is(err, ErrUnavailable) {
			t.Fatal("invalid catalog accepted")
		}
	}
	calls := 0
	c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) {
		calls++
		return response(r, 200, pageFixture([]Release{}, firstID)), nil
	})
	if _, err := c.Catalog(context.Background(), 7); !errors.Is(err, ErrUnavailable) || calls != 2 {
		t.Fatal("repeating cursor did not stop")
	}
}

func TestCatalogHTTPFailuresDoNotFollowRedirectRetryOrExposeBody(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{{401, desktopauth.ErrUnauthenticated}, {403, ErrPermission}, {404, ErrNotFound}, {503, ErrUnavailable}, {500, ErrUnavailable}, {302, ErrUnavailable}} {
		calls := 0
		c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) {
			calls++
			result := response(r, tc.status, `{"code":"private fixture-secret"}`)
			result.Header.Set("Location", "https://other.example.test/private")
			return result, nil
		})
		if _, err := c.Catalog(context.Background(), 7); !errors.Is(err, tc.want) || calls != 1 || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatalf("HTTP %d crossed rejection boundary: calls=%d error=%v", tc.status, calls, err)
		}
	}
	c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) {
		return response(r, 200, strings.Repeat("x", maxResponseBytes+1)), nil
	})
	if _, err := c.Catalog(context.Background(), 7); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unbounded response accepted")
	}
}

type revokeOnClose struct {
	io.ReadCloser
	auth *fixtureAuth
}

func (b *revokeOnClose) Close() error {
	b.auth.revoked.Store(true)
	return b.ReadCloser.Close()
}

func TestMetadataRejectsRevokedGenerationAtPublication(t *testing.T) {
	for _, artifact := range []bool{false, true} {
		a := &fixtureAuth{}
		c := fixtureClient(t, a, func(r *http.Request) (*http.Response, error) {
			body := pageFixture([]Release{releaseFixture(secondID)}, "")
			if artifact {
				data, _ := json.Marshal(artifactFixture())
				body = string(data)
			}
			result := response(r, 200, body)
			result.Body = &revokeOnClose{ReadCloser: result.Body, auth: a}
			return result, nil
		})
		var err error
		if artifact {
			_, err = c.Artifact(context.Background(), 7, secondID)
		} else {
			_, err = c.Catalog(context.Background(), 7)
		}
		if !errors.Is(err, desktopauth.ErrUnauthenticated) {
			t.Fatal("revoked completion returned usable metadata")
		}
	}
	calls := 0
	c := fixtureClient(t, &fixtureAuth{}, func(r *http.Request) (*http.Response, error) { calls++; return response(r, 200, "{}"), nil })
	for _, id := range []string{"../other", strings.ToUpper(secondID), "00000000-0000-0000-0000-000000000000"} {
		if _, err := c.Artifact(context.Background(), 7, id); !errors.Is(err, ErrNotFound) || calls != 0 {
			t.Fatal("noncanonical release input reached the API")
		}
	}
}
