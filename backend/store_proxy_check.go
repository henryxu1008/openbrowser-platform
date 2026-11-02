package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/storeproxy"
	"ant-chrome/internal/shuomiapi"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var errStoreProxyRequest = errors.New("store_proxy_check_request_invalid")

type storeProxyRequest struct {
	RequestID       string `json:"requestId"`
	ProxyConfig     string `json:"proxyConfig"`
	TargetID        string `json:"targetId"`
	CatalogVersion  string `json:"catalogVersion"`
	ExpiresAtUnixMs int64  `json:"expiresAtUnixMs"`
}
type storeProxyOptions struct {
	Available      bool                `json:"available"`
	Reason         storeproxy.Reason   `json:"reason"`
	CatalogVersion string              `json:"catalogVersion"`
	Targets        []storeproxy.Target `json:"targets"`
}

func (a *App) storeProxyOptions() storeProxyOptions {
	c := a.storeProbeCatalog
	reason := storeproxy.TargetsRequired
	if c.Available() {
		reason = storeproxy.None
	}
	return storeProxyOptions{c.Available(), reason, c.Version(), c.Targets()}
}
func validProbeID(id string) bool {
	u, e := uuid.Parse(id)
	return e == nil && u != uuid.Nil && u.String() == id
}
func decodeStoreProxyRequest(raw string) (storeProxyRequest, error) {
	var r storeProxyRequest
	if len(raw) > 16384 {
		return r, errStoreProxyRequest
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF || !validProbeID(r.RequestID) || len(r.ProxyConfig) > 8192 || len(r.TargetID) > 64 || len(r.CatalogVersion) > 64 || strings.ContainsAny(r.TargetID, "\x00\r\n@:/ ") {
		return storeProxyRequest{}, errStoreProxyRequest
	}
	return r, nil
}
func probeSession(origin, issuer, subject string, generation uint64) string {
	b, _ := json.Marshal([]string{origin, issuer, subject, strconv.FormatUint(generation, 10)})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func probeReason(err error) storeproxy.Reason {
	if err == nil {
		return storeproxy.None
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return storeproxy.Timeout
	}
	if errors.Is(err, context.Canceled) {
		return storeproxy.Canceled
	}
	if errors.Is(err, browser.ErrStoreConflict) || errors.Is(err, shuomiapi.ErrContext) {
		return storeproxy.ContextChanged
	}
	// Inspect only for fixed protocol codes. Never return, wrap or log the error.
	code := err.Error()
	if strings.Contains(code, "PERMISSION") || strings.Contains(code, "NOT_FOUND") {
		return storeproxy.PermissionDenied
	}
	return storeproxy.Unavailable
}

// StoreProxyCheck is dedicated Store authority, never a generic proxy exemption.
// Its input, credentials, underlying errors and diagnostics are never logged.
func (a *App) StoreProxyCheck(storeID, expectedVersion, raw string, revision uint64) (out string, callErr error) {
	// Do not let a library panic become a Wails error containing input/details.
	// Operation/transport cleanup defers run before this fixed error mapping.
	defer func() {
		if recover() != nil {
			out = ""
			callErr = errors.New("store_proxy_check_unavailable")
		}
	}()
	request, err := decodeStoreProxyRequest(raw)
	if err != nil {
		return "", errStoreProxyRequest
	}
	unknown := func(reason storeproxy.Reason) (string, error) {
		return storeJSON(storeproxy.Unknown(request.RequestID, request.TargetID, reason))
	}
	if storeID == "" || len(storeID) > 128 || strings.ContainsAny(storeID, "\x00\r\n") {
		return "", errStoreProxyRequest
	}
	version, err := parseStoreVersion(expectedVersion)
	if err != nil {
		return "", errStoreProxyRequest
	}
	// An IPC dispatch delayed beyond the caller's bounded intent cannot revive
	// canceled work after tombstone eviction or a suspend/resume cycle.
	now := time.Now().UnixMilli()
	if request.ExpiresAtUnixMs <= 0 || request.ExpiresAtUnixMs > now+12000 {
		return "", errStoreProxyRequest
	}
	if request.ExpiresAtUnixMs <= now {
		return unknown(storeproxy.Timeout)
	}
	dispatchContext, stopDispatch := context.WithDeadline(a.ctx, time.UnixMilli(request.ExpiresAtUnixMs))
	defer stopDispatch()
	client, dao, scope, err := a.storeScope(storeID, revision)
	if err != nil {
		return unknown(storeproxy.ContextChanged)
	}
	owner := storeproxy.Owner{Session: probeSession(scope.APIOrigin, scope.Issuer, scope.Subject, a.authGeneration), Store: storeID, Revision: revision}
	lease, err := a.storeProxyChecks.Reserve(dispatchContext, owner, request.RequestID)
	if err != nil {
		if errors.Is(err, storeproxy.ErrCanceled) {
			return unknown(storeproxy.Canceled)
		}
		if errors.Is(err, storeproxy.ErrBusy) {
			return unknown(storeproxy.Busy)
		}
		return unknown(storeproxy.Unavailable)
	}
	defer lease.Finish() // Runs after transport, watcher and lock cleanup.
	// Reserve cancellation ownership before remote admission. Even a rejected
	// busy request owns its admission context until it has actually settled.
	busy := lease.Acquire() != nil
	if a.storeGeneralMu.TryRLock() {
		defer a.storeGeneralMu.RUnlock()
	} else {
		busy = true
	}
	local := func(ctx context.Context) storeproxy.Reason {
		if a.storeCheck(revision) != nil {
			return storeproxy.ContextChanged
		}
		identity, e := client.Identity(a.authGeneration)
		if e != nil || identity.Issuer != scope.Issuer || identity.Subject != scope.Subject {
			return storeproxy.ContextChanged
		}
		if ctx.Err() != nil {
			return probeReason(ctx.Err())
		}
		return storeproxy.None
	}
	admit := func(ctx context.Context) storeproxy.Reason {
		if reason := local(ctx); reason != storeproxy.None {
			return reason
		}
		if e := client.CanUse(ctx, a.authGeneration, scope.OrganizationID); e != nil {
			if reason := local(ctx); reason != storeproxy.None {
				return reason
			}
			return probeReason(e)
		}
		if _, e := client.Store(ctx, a.authGeneration, scope.OrganizationID, storeID); e != nil {
			if reason := local(ctx); reason != storeproxy.None {
				return reason
			}
			return probeReason(e)
		}
		return local(ctx)
	}
	var binding browser.StoreBinding
	var found bool
	authorize := func(ctx context.Context) storeproxy.Reason {
		if reason := admit(ctx); reason != storeproxy.None {
			return reason
		}
		current, present, e := dao.Binding(ctx, scope)
		if e != nil {
			return storeproxy.Unavailable
		}
		if present != found || present && (current.BindingID != binding.BindingID || current.ProfileID != binding.ProfileID || current.Version != binding.Version) {
			return storeproxy.ContextChanged
		}
		return local(ctx)
	}
	stop, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-lease.Context.Done():
				return
			case <-ticker.C:
				if local(lease.Context) != storeproxy.None {
					_, _ = a.storeProxyChecks.Cancel(owner, request.RequestID)
					return
				}
			}
		}
	}()
	defer func() { close(stop); <-stopped }()
	if reason := admit(lease.Context); reason != storeproxy.None {
		return unknown(reason)
	}
	// Do not disclose busy or binding state before Store/permission admission.
	if busy {
		return unknown(storeproxy.Busy)
	}
	binding, found, err = dao.Binding(lease.Context, scope)
	if err != nil {
		return unknown(storeproxy.Unavailable)
	}
	if found && binding.Version != version || !found && version != 0 {
		return unknown(storeproxy.ContextChanged)
	}
	if reason := authorize(lease.Context); reason != storeproxy.None {
		return unknown(reason)
	}
	catalog := a.storeProbeCatalog
	if !catalog.Available() {
		return unknown(storeproxy.TargetsRequired)
	}
	target, ok := catalog.Resolve(request.TargetID, request.CatalogVersion)
	if !ok {
		return unknown(storeproxy.ContextChanged)
	}
	runner := a.storeProbeRunner
	if runner == nil {
		runner = storeproxy.NetworkProber{}
	}
	result := runner.Run(lease.Context, request.ProxyConfig, target, request.RequestID, request.TargetID, authorize)
	// Run returns only after its owned transports/connection bodies have closed.
	if reason := authorize(lease.Context); reason != storeproxy.None {
		return unknown(reason)
	}
	return storeJSON(result)
}

// Cancel is owned cleanup: it may stop an original revision after enterprise
// selection changes, but cannot grant/check/start anything in a new revision.
func (a *App) StoreProxyCheckCancel(storeID, requestID string, revision uint64) (string, error) {
	if !validProbeID(requestID) || storeID == "" || len(storeID) > 128 || strings.ContainsAny(storeID, "\x00\r\n") {
		return "", errStoreProxyRequest
	}
	client, _, err := a.storeClient()
	if err != nil {
		return "", errors.New("store_proxy_check_unavailable")
	}
	identity, err := client.Identity(a.authGeneration)
	if err != nil {
		return "", errors.New("store_proxy_check_context_changed")
	}
	owner := storeproxy.Owner{Session: probeSession(client.Origin(), identity.Issuer, identity.Subject, a.authGeneration), Store: storeID, Revision: revision}
	done, err := a.storeProxyChecks.Cancel(owner, requestID)
	if err != nil {
		return "", errors.New("store_proxy_check_unavailable")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 12*time.Second)
	defer cancel()
	settled := false
	select {
	case <-done:
		settled = true
	case <-ctx.Done():
	}
	return storeJSON(struct {
		RequestID string `json:"requestId"`
		Settled   bool   `json:"settled"`
	}{requestID, settled})
}
