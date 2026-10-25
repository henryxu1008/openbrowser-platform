package browser

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var ErrStoreConflict = errors.New("BINDING_VERSION_CONFLICT")
var ErrStoreUnavailable = errors.New("store_environment_unavailable")
var ErrStoreUnknown = errors.New("store_operation_result_unknown")

type StoreScope struct{ APIOrigin, Issuer, Subject, OrganizationID, StoreID string }

func (s StoreScope) args() []any {
	return []any{s.APIOrigin, s.Issuer, s.Subject, s.OrganizationID, s.StoreID}
}
func (s StoreScope) valid() bool {
	p, e := uuid.Parse(s.StoreID)
	return e == nil && p != uuid.Nil && p.String() == s.StoreID && s.APIOrigin != "" && s.Issuer != "" && s.Subject != "" && s.OrganizationID != ""
}

type StoreBinding struct {
	BindingID      string `json:"bindingId"`
	ProfileID      string `json:"profileId"`
	Version        int64  `json:"-"`
	BindingVersion string `json:"bindingVersion"`
}
type StoreOperation struct {
	OperationID        string `json:"operationId"`
	BindingID          string `json:"bindingId"`
	ProfileID          string `json:"profileId"`
	BindingVersion     int64  `json:"-"`
	Action             string `json:"action"`
	Phase              string `json:"phase"`
	Attempted          bool   `json:"attempted"`
	Outcome            string `json:"outcome"`
	ReasonCode         string `json:"reasonCode"`
	ObservedAt         string `json:"observedAt"`
	ReportState        string `json:"reportState"`
	RuntimeObservation string `json:"runtimeObservation"`
	RuntimeObservedAt  string `json:"runtimeObservedAt"`
}
type StoreDAO struct {
	db      *sql.DB
	present atomic.Bool
}

func NewStoreDAO(db *sql.DB) (*StoreDAO, error) {
	if db == nil {
		return nil, ErrStoreUnavailable
	}
	d := &StoreDAO{db: db}
	var count int
	if db.QueryRow("SELECT (SELECT count(*) FROM browser_store_bindings)+(SELECT count(*) FROM browser_store_operations)").Scan(&count) != nil {
		return nil, ErrStoreUnavailable
	}
	d.present.Store(count > 0)
	return d, nil
}
func (d *StoreDAO) HasManagedData() bool { return d != nil && d.present.Load() }

const storeWhere = "api_origin=? AND issuer=? AND subject=? AND organization_id=? AND store_id=?"

func (d *StoreDAO) Binding(ctx context.Context, s StoreScope) (StoreBinding, bool, error) {
	if !s.valid() {
		return StoreBinding{}, false, ErrStoreUnavailable
	}
	var b StoreBinding
	err := d.db.QueryRowContext(ctx, "SELECT binding_id,profile_id,binding_version FROM browser_store_bindings WHERE "+storeWhere, s.args()...).Scan(&b.BindingID, &b.ProfileID, &b.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return b, false, nil
	}
	if err != nil {
		return b, false, ErrStoreUnavailable
	}
	b.BindingVersion = strconv.FormatInt(b.Version, 10)
	return b, true, nil
}
func (d *StoreDAO) Prepare(ctx context.Context, s StoreScope, b StoreBinding, key, action string) error {
	p, e := uuid.Parse(key)
	if !s.valid() || e != nil || p == uuid.Nil || p.String() != key || b.Version < 1 || (action != "configure_environment" && action != "open_environment") {
		return ErrStoreUnavailable
	}
	args := append([]any{key}, s.args()...)
	args = append(args, b.BindingID, b.ProfileID, b.Version, action, "prepared", time.Now().UTC().Format(time.RFC3339Nano))
	_, err := d.db.ExecContext(ctx, "INSERT INTO browser_store_operations(operation_id,api_origin,issuer,subject,organization_id,store_id,binding_id,profile_id,binding_version,action,phase,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", args...)
	if err != nil {
		return ErrStoreConflict
	}
	d.present.Store(true)
	return nil
}
func (d *StoreDAO) Phase(ctx context.Context, s StoreScope, key, phase string) error {
	if phase != "admitted" && phase != "attempted" {
		return ErrStoreUnavailable
	}
	prior := "prepared"
	if phase == "attempted" {
		prior = "admitted"
	}
	args := []any{phase, phase == "attempted", key}
	args = append(args, s.args()...)
	args = append(args, prior)
	r, err := d.db.ExecContext(ctx, "UPDATE browser_store_operations SET phase=?,attempted=? WHERE operation_id=? AND "+storeWhere+" AND phase=?", args...)
	if err != nil {
		return ErrStoreUnavailable
	}
	n, e := r.RowsAffected()
	if e != nil || n != 1 {
		return ErrStoreUnknown
	}
	return nil
}
func (d *StoreDAO) Finish(ctx context.Context, s StoreScope, key, outcome, reason string) error {
	if outcome != "succeeded" && outcome != "failed" && outcome != "unknown" {
		return ErrStoreUnavailable
	}
	args := []any{outcome, outcome, reason, time.Now().UTC().Format(time.RFC3339Nano), key}
	args = append(args, s.args()...)
	r, err := d.db.ExecContext(ctx, "UPDATE browser_store_operations SET phase=?,outcome=?,reason_code=?,observed_at=? WHERE operation_id=? AND "+storeWhere+" AND phase IN ('prepared','admitted','attempted')", args...)
	if err != nil {
		return ErrStoreUnavailable
	}
	n, e := r.RowsAffected()
	if e != nil || n != 1 {
		return ErrStoreUnknown
	}
	return nil
}
func (d *StoreDAO) Reported(ctx context.Context, s StoreScope, key string) error {
	args := []any{key}
	args = append(args, s.args()...)
	_, err := d.db.ExecContext(ctx, "UPDATE browser_store_operations SET report_state='confirmed' WHERE operation_id=? AND "+storeWhere, args...)
	if err != nil {
		return ErrStoreUnavailable
	}
	return nil
}
func (d *StoreDAO) Operation(ctx context.Context, s StoreScope, key string) (StoreOperation, error) {
	var o StoreOperation
	args := []any{key}
	args = append(args, s.args()...)
	err := d.db.QueryRowContext(ctx, "SELECT operation_id,binding_id,profile_id,binding_version,action,phase,attempted,outcome,reason_code,observed_at,report_state,runtime_observation,runtime_observed_at FROM browser_store_operations WHERE operation_id=? AND "+storeWhere, args...).Scan(&o.OperationID, &o.BindingID, &o.ProfileID, &o.BindingVersion, &o.Action, &o.Phase, &o.Attempted, &o.Outcome, &o.ReasonCode, &o.ObservedAt, &o.ReportState, &o.RuntimeObservation, &o.RuntimeObservedAt)
	if err != nil {
		return o, ErrStoreUnavailable
	}
	return o, nil
}
func (d *StoreDAO) Pending(ctx context.Context, s StoreScope) (*StoreOperation, error) {
	var key string
	err := d.db.QueryRowContext(ctx, "SELECT operation_id FROM browser_store_operations WHERE "+storeWhere+" AND phase IN ('prepared','admitted','attempted','unknown') LIMIT 1", s.args()...).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrStoreUnavailable
	}
	o, e := d.Operation(ctx, s, key)
	return &o, e
}

// BuildStoreProfile reuses original normalization but publishes no state. Store
// identity and paths are generated in Go, never adopted from a profile name.
func (m *Manager) BuildStoreProfile(input ProfileInput, b StoreBinding) (*Profile, error) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if len(input.LaunchArgs) > 0 || input.UserDataDir != "" {
		return nil, ErrStoreUnavailable
	}
	// Shared legacy proxy records carry no Shuomi owner. Reuse direct/local proxy
	// input; do not silently adopt another login's pool credential.
	if input.ProxyId != "" && input.ProxyId != directProxyID {
		return nil, ErrStoreUnavailable
	}
	input.ProxyId = ""
	input.UserDataDir = b.ProfileID
	input.Tags = nil
	input.Keywords = nil
	input.GroupId = ""
	candidate, err := m.buildProfileLocked(input, b.ProfileID)
	if err != nil {
		return nil, ErrStoreUnavailable
	}
	// The managed use case validates these arguments against the original
	// capability descriptor. Do not bind a same-valued legacy pool credential.
	candidate.ProxyId = ""
	ClearProfileProxyBinding(candidate)
	if candidate.RestoreLastSession == "" {
		candidate.RestoreLastSession = RestoreLastSessionDisabled
		if m.Config.Browser.RestoreLastSession {
			candidate.RestoreLastSession = RestoreLastSessionEnabled
		}
	}
	if old := m.Profiles[b.ProfileID]; old != nil {
		candidate.CreatedAt = old.CreatedAt
		if old.Running {
			return nil, ErrStoreConflict
		}
	}
	return candidate, nil
}

// SaveStoreProfile commits Profile, immutable binding and the journal's result
// phase together. The pre-existing PREPARED intent survives a failed commit.
func (m *Manager) SaveStoreProfile(ctx context.Context, d *StoreDAO, s StoreScope, b StoreBinding, expected int64, key string, p *Profile, check func() error) error {
	if !s.valid() || p == nil || p.ProfileId != b.ProfileID || p.UserDataDir != b.ProfileID || check == nil {
		return ErrStoreUnavailable
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if err := check(); err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrStoreUnavailable
	}
	defer tx.Rollback()
	// The existing Profile FK is immediate. Both rows remain private to this
	// transaction, and a binding/CAS failure rolls the Profile write back too.
	if err = upsertProfile(tx, p); err != nil {
		return ErrStoreUnavailable
	}
	if expected == 0 {
		args := append([]any{b.BindingID, b.ProfileID}, s.args()...)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		args = append(args, b.Version, key, now, now)
		_, err = tx.ExecContext(ctx, "INSERT INTO browser_store_bindings(binding_id,profile_id,api_origin,issuer,subject,organization_id,store_id,binding_version,create_operation_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", args...)
	} else {
		args := []any{b.Version, time.Now().UTC().Format(time.RFC3339Nano)}
		args = append(args, s.args()...)
		args = append(args, b.BindingID, b.ProfileID, expected)
		var result sql.Result
		result, err = tx.ExecContext(ctx, "UPDATE browser_store_bindings SET binding_version=?,updated_at=? WHERE "+storeWhere+" AND binding_id=? AND profile_id=? AND binding_version=?", args...)
		if err == nil {
			n, e := result.RowsAffected()
			if e != nil || n != 1 {
				err = ErrStoreConflict
			}
		}
	}
	if err != nil {
		return ErrStoreConflict
	}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano), key}
	args = append(args, s.args()...)
	args = append(args, b.BindingID, b.ProfileID, b.Version)
	result, err := tx.ExecContext(ctx, "UPDATE browser_store_operations SET phase='succeeded',outcome='succeeded',reason_code='configuration_saved',observed_at=? WHERE operation_id=? AND "+storeWhere+" AND phase='admitted' AND attempted=0 AND action='configure_environment' AND binding_id=? AND profile_id=? AND binding_version=?", args...)
	if err != nil {
		return ErrStoreUnavailable
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return ErrStoreConflict
	}
	if err = check(); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return ErrStoreUnknown
	}
	m.Profiles[p.ProfileId] = p
	return nil
}
