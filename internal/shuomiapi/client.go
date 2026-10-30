// Package shuomiapi is the Go-only adapter for the existing Shuomi Store API.
package shuomiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"ant-chrome/internal/apiconfig"
	"ant-chrome/internal/desktopauth"
	"github.com/google/uuid"
)

var ErrConfiguration = errors.New("store_service_configuration_required")
var ErrUnavailable = errors.New("store_service_unavailable")
var ErrContext = errors.New("store_context_changed")

// Only a validated non-success response is a definite admission rejection.
// Transport errors, malformed envelopes, 5xx and stale identity remain unknown.
type Rejection struct {
	Code       string
	StatusCode int
}

func (e *Rejection) Error() string {
	if e.StatusCode == 0 {
		return e.Code
	}
	// Only the fixed protocol code and HTTP status cross the native bridge.
	// Never include response messages, headers, URLs or transport credentials.
	return fmt.Sprintf("%s (store_api_response=%d)", e.Code, e.StatusCode)
}
func IsRejected(err error) bool { var rejection *Rejection; return errors.As(err, &rejection) }

func rejectionStatus(code string) int {
	switch code {
	case "AUTHENTICATION_REQUIRED":
		return http.StatusUnauthorized
	case "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED":
		return http.StatusForbidden
	case "ORGANIZATION_CONTEXT_CHANGED", "OPERATION_CONFLICT", "BINDING_VERSION_CONFLICT":
		return http.StatusConflict
	case "STORE_OR_OPERATION_NOT_FOUND":
		return http.StatusNotFound
	case "INVALID_REQUEST":
		return http.StatusBadRequest
	default:
		return 0
	}
}

type Auth interface {
	Check(uint64) error
	Status() desktopauth.Status
	AuthorizeRequest(uint64, *http.Request) error
}
type Config apiconfig.Config

func LoadConfig(root string) (Config, error) {
	cfg, err := apiconfig.Load(root)
	if err != nil {
		return Config{}, ErrConfiguration
	}
	return Config(cfg), nil
}
func (c Config) valid() bool {
	return apiconfig.Config(c).Valid()
}

type Client struct {
	cfg  Config
	auth Auth
	http *http.Client
}

func New(cfg Config, auth Auth) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	return newClient(cfg, auth, &http.Client{Transport: transport})
}
func newClient(cfg Config, auth Auth, client *http.Client) (*Client, error) {
	if !cfg.valid() || auth == nil || client == nil {
		return nil, ErrConfiguration
	}
	copy := *client
	copy.Timeout = 15 * time.Second
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{cfg: cfg, auth: auth, http: &copy}, nil
}
func (c *Client) Origin() string { return c.cfg.APIOrigin }
func (c *Client) Identity(g uint64) (desktopauth.Identity, error) {
	if err := c.auth.Check(g); err != nil {
		return desktopauth.Identity{}, err
	}
	s := c.auth.Status()
	if s.Generation != g || s.State != "signed_in" || s.Subject == "" || s.Issuer == "" {
		return desktopauth.Identity{}, ErrContext
	}
	return desktopauth.Identity{Issuer: s.Issuer, Subject: s.Subject}, nil
}
func (c *Client) request(ctx context.Context, g uint64, org, method, path, key string, input, output any) error {
	if _, err := c.Identity(g); err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil || len(data) > 16384 {
			return ErrUnavailable
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.APIOrigin+"/api/v1/workbench/"+path, body)
	if err != nil {
		return ErrUnavailable
	}
	req.Close = true
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if org != "" {
		req.Header.Set("X-Requested-Organization-ID", org)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if err = c.auth.AuthorizeRequest(g, req); err != nil {
		return err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return ErrUnavailable
	}
	if err = c.auth.Check(g); err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &failure)
		if expected := rejectionStatus(failure.Code); expected != 0 && response.StatusCode == expected {
			return &Rejection{Code: failure.Code, StatusCode: response.StatusCode}
		}
		return ErrUnavailable
	}
	if json.Unmarshal(raw, output) != nil {
		return ErrUnavailable
	}
	return nil
}

type Organization struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}
type WorkbenchContext struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	EffectiveOrganizationID *string        `json:"effectiveOrganizationId"`
	SelectionRequired       bool           `json:"selectionRequired"`
	Organizations           []Organization `json:"organizations"`
}

func (c *Client) Context(ctx context.Context, g uint64, org string) (WorkbenchContext, error) {
	var out WorkbenchContext
	err := c.request(ctx, g, org, "GET", "context", "", nil, &out)
	if err != nil {
		return out, err
	}
	identity, err := c.Identity(g)
	if err != nil || out.User.ID != identity.Subject {
		return WorkbenchContext{}, ErrContext
	}
	if org != "" && (out.EffectiveOrganizationID == nil || *out.EffectiveOrganizationID != org) {
		return WorkbenchContext{}, ErrContext
	}
	return out, nil
}
func (c *Client) SelectOrganization(ctx context.Context, g uint64, org string) (WorkbenchContext, error) {
	if org == "" || len(org) > 200 {
		return WorkbenchContext{}, ErrContext
	}
	var out WorkbenchContext
	err := c.request(ctx, g, "", "PUT", "context/effective-organization", "", struct {
		OrganizationID string `json:"organizationId"`
	}{org}, &out)
	if err != nil {
		return out, err
	}
	id, err := c.Identity(g)
	if err != nil || out.User.ID != id.Subject || out.EffectiveOrganizationID == nil || *out.EffectiveOrganizationID != org {
		return WorkbenchContext{}, ErrContext
	}
	return out, nil
}
func (c *Client) CanUse(ctx context.Context, g uint64, org string) error {
	v, err := c.Context(ctx, g, org)
	if err != nil {
		return err
	}
	for _, o := range v.Organizations {
		if o.ID == org {
			for _, p := range o.Permissions {
				if p == "workbench.store.browser.use" {
					return nil
				}
			}
		}
	}
	return errors.New("PERMISSION_DENIED")
}

type Store struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Platform         string          `json:"platform"`
	Region           string          `json:"region"`
	RecordStatus     string          `json:"recordStatus"`
	ServiceStatus    *string         `json:"serviceStatus"`
	ConnectionStatus json.RawMessage `json:"connectionStatus"`
}
type Catalog struct {
	Items      []Store `json:"items"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}
type Envelope[T any] struct {
	SchemaVersion  string `json:"schemaVersion"`
	UserID         string `json:"userId"`
	OrganizationID string `json:"organizationId"`
	Data           T      `json:"data"`
}

func (c *Client) verify(g uint64, org string, user, organization string) error {
	id, err := c.Identity(g)
	if err != nil || org == "" || organization != org || id.Subject != user {
		return ErrContext
	}
	return nil
}
func (c *Client) Catalog(ctx context.Context, g uint64, org, search, platform string, page int) (Catalog, error) {
	if page < 1 || page > 1000000 || len(search) > 200 || len(platform) > 32 {
		return Catalog{}, ErrUnavailable
	}
	q := url.Values{"page": {strconv.Itoa(page)}, "pageSize": {"20"}}
	if search != "" {
		q.Set("q", search)
	}
	if platform != "" {
		q.Set("platform", platform)
	}
	var out Envelope[Catalog]
	err := c.request(ctx, g, org, "GET", "store-catalog?"+q.Encode(), "", nil, &out)
	if err != nil {
		return Catalog{}, err
	}
	if out.SchemaVersion != "store-center-v1" || c.verify(g, org, out.UserID, out.OrganizationID) != nil {
		return Catalog{}, ErrContext
	}
	seen := map[string]bool{}
	if out.Data.Pagination.Page != page || out.Data.Pagination.PageSize != 20 || out.Data.Pagination.Total < int64(len(out.Data.Items)) || len(out.Data.Items) > 20 {
		return Catalog{}, ErrContext
	}
	for _, s := range out.Data.Items {
		if !canonicalID(s.ID) || seen[s.ID] {
			return Catalog{}, ErrContext
		}
		seen[s.ID] = true
	}
	return out.Data, nil
}
func canonicalID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil && parsed.String() == id
}
func (c *Client) Store(ctx context.Context, g uint64, org, id string) (Store, error) {
	if !canonicalID(id) {
		return Store{}, ErrContext
	}
	var out Envelope[Store]
	err := c.request(ctx, g, org, "GET", "stores/"+id+"/summary", "", nil, &out)
	if err != nil {
		return Store{}, err
	}
	if out.SchemaVersion != "store-center-v1" || out.Data.ID != id || c.verify(g, org, out.UserID, out.OrganizationID) != nil {
		return Store{}, ErrContext
	}
	return out.Data, nil
}

type Command struct {
	SchemaVersion  string `json:"schemaVersion"`
	Action         string `json:"action"`
	BindingID      string `json:"bindingId"`
	ProfileID      string `json:"profileId"`
	BindingVersion string `json:"bindingVersion"`
}
type Report struct {
	SchemaVersion string    `json:"schemaVersion"`
	Kind          string    `json:"kind"`
	Outcome       string    `json:"outcome"`
	ReasonCode    string    `json:"reasonCode"`
	ObservedAt    time.Time `json:"observedAt"`
}
type Receipt struct {
	SchemaVersion       string    `json:"schemaVersion"`
	UserID              string    `json:"userId"`
	OrganizationID      string    `json:"organizationId"`
	StoreID             string    `json:"storeId"`
	OperationID         string    `json:"operationId"`
	Action              string    `json:"action"`
	BindingID           string    `json:"bindingId"`
	ProfileID           string    `json:"profileId"`
	BindingVersion      string    `json:"bindingVersion"`
	ExecutionAuthorized bool      `json:"executionAuthorized"`
	ExecutionWindowMS   int       `json:"executionWindowMs"`
	AdmittedAt          time.Time `json:"admittedAt"`
	Reports             []Report  `json:"reports"`
}

func (c *Client) Admit(ctx context.Context, g uint64, org, store, key string, command Command) (Receipt, error) {
	if !canonicalID(store) || !canonicalID(key) {
		return Receipt{}, ErrContext
	}
	var out Receipt
	err := c.request(ctx, g, org, "POST", "stores/"+store+"/browser-operations", key, command, &out)
	if err != nil {
		return out, err
	}
	if c.verify(g, org, out.UserID, out.OrganizationID) != nil || out.SchemaVersion != "store-browser-v1" || out.StoreID != store || out.OperationID != key || out.Action != command.Action || out.BindingID != command.BindingID || out.ProfileID != command.ProfileID || out.BindingVersion != command.BindingVersion || out.ExecutionWindowMS != 15000 {
		return Receipt{}, ErrContext
	}
	return out, nil
}
func (c *Client) Receipt(ctx context.Context, g uint64, org, store, key string) (Receipt, error) {
	if !canonicalID(store) || !canonicalID(key) {
		return Receipt{}, ErrContext
	}
	var out Receipt
	err := c.request(ctx, g, "", "GET", "stores/"+store+"/browser-operations/"+key, "", nil, &out)
	if err != nil {
		return out, err
	}
	if c.verify(g, org, out.UserID, out.OrganizationID) != nil || out.SchemaVersion != "store-browser-v1" || out.StoreID != store || out.OperationID != key || out.ExecutionAuthorized {
		return Receipt{}, ErrContext
	}
	return out, nil
}
func (c *Client) Report(ctx context.Context, g uint64, org, store, key string, report Report) (Receipt, error) {
	if !canonicalID(store) || !canonicalID(key) {
		return Receipt{}, ErrContext
	}
	var out Receipt
	err := c.request(ctx, g, "", "POST", "stores/"+store+"/browser-operations/"+key+"/reports", "", report, &out)
	if err != nil {
		return out, err
	}
	if c.verify(g, org, out.UserID, out.OrganizationID) != nil || out.SchemaVersion != "store-browser-v1" || out.StoreID != store || out.OperationID != key || out.ExecutionAuthorized {
		return Receipt{}, ErrContext
	}
	return out, nil
}
