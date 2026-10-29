package shuomiapi

import (
	"ant-chrome/internal/desktopauth"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type authFixture struct{ revoked bool }

func (a *authFixture) Check(g uint64) error {
	if a.revoked || g != 7 {
		return desktopauth.ErrUnauthenticated
	}
	return nil
}
func (a *authFixture) Status() desktopauth.Status {
	return desktopauth.Status{Generation: 7, State: "signed_in", Issuer: "https://identity.example.test", Subject: "subject-a", ExpiresAt: time.Now().Add(time.Hour)}
}
func (a *authFixture) AuthorizeRequest(g uint64, r *http.Request) error {
	if err := a.Check(g); err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer fixture-never-render")
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStoreClientIsOriginBoundAndDoesNotFollowRedirectOrRetry(t *testing.T) {
	a := &authFixture{}
	calls := 0
	client, err := newClient(Config{APIOrigin: "https://api.example.test"}, a, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.example.test" || r.Header.Get("Authorization") != "Bearer fixture-never-render" || !r.Close {
			t.Fatal("unsafe request")
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://other.example.test"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Context(context.Background(), 7, ""); err == nil || calls != 1 {
		t.Fatalf("redirect/retry: %d %v", calls, err)
	}
}
func TestStoreClientRejectsIdentityMismatchAndRevokedCompletion(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "wrong_subject", true: "revoked_completion"}[revoke], func(t *testing.T) {
			a := &authFixture{}
			client, err := newClient(Config{APIOrigin: "https://api.example.test"}, a, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if revoke {
					a.revoked = true
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"user":{"id":"different"},"effectiveOrganizationId":"org-a","organizations":[]}`))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.Context(context.Background(), 7, "org-a"); err == nil {
				t.Fatal("stale/unverified context published")
			}
		})
	}
}
func TestStoreClientSanitizesTransportFailure(t *testing.T) {
	a := &authFixture{}
	calls := 0
	client, _ := newClient(Config{APIOrigin: "https://api.example.test"}, a, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("fixture-never-render private transport detail")
	})})
	_, err := client.Context(context.Background(), 7, "")
	if err == nil || strings.Contains(err.Error(), "fixture-never-render") || calls != 1 {
		t.Fatalf("unsanitized/repeated transport: %v count=%d", err, calls)
	}
}

func TestStoreClientDistinguishesKnownRejectionFromAmbiguousFailure(t *testing.T) {
	for _, tc := range []struct {
		status   int
		body     string
		rejected bool
	}{{403, `{"code":"PERMISSION_DENIED"}`, true}, {400, `{"code":"INVALID_REQUEST"}`, true}, {503, `{"code":"PERMISSION_DENIED"}`, false}, {403, `{"code":"private arbitrary detail"}`, false}, {503, ``, false}} {
		client, err := newClient(Config{APIOrigin: "https://api.example.test"}, &authFixture{}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Context(context.Background(), 7, "")
		if err == nil || IsRejected(err) != tc.rejected {
			t.Fatalf("rejection/unknown conflated: %d %v", tc.status, err)
		}
	}
	for _, origin := range []string{"https://api.example.test#", "https://api.example.test?", "https://api.example.test\\path"} {
		if (Config{APIOrigin: origin}).valid() {
			t.Fatal("invalid origin accepted")
		}
	}
}
func TestStorePublicConfigStrictlyRejectsExtraDuplicateOrUnsafeOrigins(t *testing.T) {
	root := t.TempDir()
	for _, body := range []string{`{"apiOrigin":"https://api.example.test","apiOrigin":"https://other.example.test"}`, `{"apiOrigin":"http://api.example.test"}`, `{"apiOrigin":"https://user:private@api.example.test"}`, `{"apiOrigin":"https://api.example.test/path"}`, `{"apiOrigin":"https://api.example.test","secret":"forbidden"}`, strings.Repeat(" ", 4097)} {
		if err := os.WriteFile(filepath.Join(root, "shuomi-public.json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfig(root); !errors.Is(err, ErrConfiguration) {
			t.Fatal("unsafe public config accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "shuomi-public.json"), []byte(`{"apiOrigin":"https://api.example.test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(root); err != nil {
		t.Fatal(err)
	}
}

func TestStoreClientAuthenticationRejectionRetainsSafeHTTPStage(t *testing.T) {
	auth := &authFixture{}
	calls := 0
	client, err := newClient(Config{APIOrigin: "https://api.example.test"}, auth, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"code":"AUTHENTICATION_REQUIRED","message":"response detail must stay private"}`))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Context(context.Background(), 7, "")
	if err == nil || err.Error() != "AUTHENTICATION_REQUIRED (store_api_response=401)" {
		t.Fatalf("remote authentication rejection lost safe response stage: %v", err)
	}
	if !IsRejected(err) || auth.Check(7) != nil || calls != 1 {
		t.Fatal("remote rejection changed local identity or retried the request")
	}
	var rejected *Rejection
	if !errors.As(err, &rejected) || rejected.StatusCode != http.StatusUnauthorized {
		t.Fatal("remote authentication status was not retained in the Go-only error")
	}
}

func TestStoreClientUnknownResponseCannotBecomeDefiniteRejection(t *testing.T) {
	for _, status := range []int{0, 400, 401, 403, 404, 409, 503} {
		client, err := newClient(Config{APIOrigin: "https://api.example.test"}, &authFixture{}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"code":"UNRECOGNIZED"}`))}, nil
		})})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Context(context.Background(), 7, "")
		if !errors.Is(err, ErrUnavailable) || IsRejected(err) {
			t.Fatal("unrecognized response code became a definite rejection")
		}
	}
}

func TestStoreClientRejectsMismatchedHTTPStatusAndProtocolCode(t *testing.T) {
	contracts := map[string]int{
		"AUTHENTICATION_REQUIRED":      http.StatusUnauthorized,
		"PERMISSION_DENIED":            http.StatusForbidden,
		"ORGANIZATION_ACCESS_REVOKED":  http.StatusForbidden,
		"ORGANIZATION_CONTEXT_CHANGED": http.StatusConflict,
		"STORE_OR_OPERATION_NOT_FOUND": http.StatusNotFound,
		"OPERATION_CONFLICT":           http.StatusConflict,
		"BINDING_VERSION_CONFLICT":     http.StatusConflict,
		"INVALID_REQUEST":              http.StatusBadRequest,
	}
	for code, expectedStatus := range contracts {
		for _, status := range []int{400, 401, 403, 404, 409, 503} {
			t.Run(code+"/"+http.StatusText(status), func(t *testing.T) {
				calls := 0
				client, err := newClient(Config{APIOrigin: "https://api.example.test"}, &authFixture{}, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"code":"` + code + `"}`))}, nil
				})})
				if err != nil {
					t.Fatal(err)
				}
				_, err = client.Context(context.Background(), 7, "")
				if err == nil || IsRejected(err) != (status == expectedStatus) || calls != 1 {
					t.Fatal("status/code mismatch accepted as definite rejection or request retried")
				}
				if status != expectedStatus && !errors.Is(err, ErrUnavailable) {
					t.Fatal("non-contract response did not stay unavailable/unknown")
				}
			})
		}
	}
}
