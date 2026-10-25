package desktopauth

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type loginDiagnosticTransport func(*http.Request) (*http.Response, error)

func (f loginDiagnosticTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise the production Login entrypoint without a listener, browser or IdP.
// No failure response, URL or transport error may become a renderer diagnostic.
func TestLoginDiscoveryFailureKeepsSafeStageAndNeverOpensBrowser(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status           int
		transportError   bool
	}{
		{"transport", "", "identity_discovery_request_failed", 0, true},
		{"http_status", "private-provider-body", "identity_discovery_request_failed", 503, false},
		{"malformed_document", "private-provider-body", "identity_discovery_document_invalid", 200, false},
		{"incompatible_metadata", `{}`, "identity_discovery_metadata_invalid", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, opened := 0, 0
			client := &http.Client{Transport: loginDiagnosticTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Method != "GET" || r.URL.Path != "/.well-known/openid-configuration" {
					t.Fatalf("unexpected request shape: %s %s", r.Method, r.URL.Path)
				}
				if tc.transportError {
					return nil, errors.New("private-transport-detail")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{}}, nil
			})}
			m := newManager(Config{Issuer: "https://provider.invalid", ClientID: "synthetic-public"}, client, time.Now)
			defer m.Close()
			status, err := m.Login(m.Status().Generation, func(string) error { opened++; return nil })
			if !errors.Is(err, ErrLogin) || status.State != "signed_out" || status.ErrorCode != tc.code {
				t.Fatalf("failure lost its safe stage: status=%+v err=%v; want %s", status, err, tc.code)
			}
			if requests != 1 || opened != 0 || m.pending != nil || status.APIGranted || m.Check(status.Generation) == nil {
				t.Fatalf("failed login must remain closed, with one request and no browser: requests=%d opened=%d", requests, opened)
			}
			// Polling observes the same failure; it must never restart a login.
			if m.Status().ErrorCode != tc.code || requests != 1 {
				t.Fatal("status polling changed or replayed the failed login")
			}
		})
	}
}

func TestLoginStaleGenerationNeverDispatchesDiscovery(t *testing.T) {
	requests, opened := 0, 0
	m := newManager(Config{Issuer: "https://provider.invalid", ClientID: "synthetic-public"}, &http.Client{Transport: loginDiagnosticTransport(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("unexpected dispatch")
	})}, time.Now)
	defer m.Close()
	old := m.Status().Generation
	m.Logout()
	status, err := m.Login(old, func(string) error { opened++; return nil })
	if !errors.Is(err, ErrCancelled) || requests != 0 || opened != 0 || status.ErrorCode != "" || status.State != "signed_out" {
		t.Fatalf("stale generation was dispatched or misreported: status=%+v requests=%d opened=%d err=%v", status, requests, opened, err)
	}
}
