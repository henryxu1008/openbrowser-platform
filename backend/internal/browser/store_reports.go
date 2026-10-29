package browser

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type StoreReport struct {
	Kind       string `json:"kind"`
	Outcome    string `json:"outcome"`
	ReasonCode string `json:"reasonCode"`
	ObservedAt string `json:"observedAt"`
	Confirmed  bool   `json:"confirmed"`
}

func (d *StoreDAO) SaveReport(ctx context.Context, s StoreScope, key string, r StoreReport) error {
	if _, err := d.Operation(ctx, s, key); err != nil {
		return err
	}
	if r.Kind != "configure_result" && r.Kind != "open_result" && r.Kind != "close_result" {
		return ErrStoreUnavailable
	}
	_, err := d.db.ExecContext(ctx, "INSERT INTO browser_store_reports(operation_id,kind,outcome,reason_code,observed_at) VALUES(?,?,?,?,?) ON CONFLICT(operation_id,kind) DO NOTHING", key, r.Kind, r.Outcome, r.ReasonCode, r.ObservedAt)
	if err != nil {
		return ErrStoreUnavailable
	}
	var old StoreReport
	if err = d.db.QueryRowContext(ctx, "SELECT outcome,reason_code,observed_at FROM browser_store_reports WHERE operation_id=? AND kind=?", key, r.Kind).Scan(&old.Outcome, &old.ReasonCode, &old.ObservedAt); err != nil {
		return ErrStoreUnavailable
	}
	if old.Outcome != r.Outcome || old.ReasonCode != r.ReasonCode || old.ObservedAt != r.ObservedAt {
		return ErrStoreConflict
	}
	return nil
}
func (d *StoreDAO) ConfirmReport(ctx context.Context, s StoreScope, key, kind string) error {
	if _, err := d.Operation(ctx, s, key); err != nil {
		return err
	}
	_, err := d.db.ExecContext(ctx, "UPDATE browser_store_reports SET confirmed=1 WHERE operation_id=? AND kind=?", key, kind)
	if err != nil {
		return ErrStoreUnavailable
	}
	if kind != "close_result" {
		return d.Reported(ctx, s, key)
	}
	return nil
}
func (d *StoreDAO) Latest(ctx context.Context, s StoreScope) (*StoreOperation, error) {
	var key string
	err := d.db.QueryRowContext(ctx, "SELECT operation_id FROM browser_store_operations WHERE "+storeWhere+" ORDER BY created_at DESC,operation_id DESC LIMIT 1", s.args()...).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrStoreUnavailable
	}
	o, e := d.Operation(ctx, s, key)
	return &o, e
}
func (d *StoreDAO) LastOpen(ctx context.Context, s StoreScope) (StoreOperation, error) {
	o, err := d.LatestOpen(ctx, s)
	if err != nil {
		return StoreOperation{}, err
	}
	if o == nil {
		return StoreOperation{}, ErrStoreUnavailable
	}
	return *o, nil
}
func (d *StoreDAO) LatestOpen(ctx context.Context, s StoreScope) (*StoreOperation, error) {
	var key string
	err := d.db.QueryRowContext(ctx, "SELECT operation_id FROM browser_store_operations WHERE "+storeWhere+" AND action='open_environment' AND attempted=1 ORDER BY created_at DESC,operation_id DESC LIMIT 1", s.args()...).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrStoreUnavailable
	}
	o, err := d.Operation(ctx, s, key)
	return &o, err
}

func (d *StoreDAO) ObserveRuntime(ctx context.Context, s StoreScope, key, observation string) error {
	if observation != "no_directory_process_observed" {
		return ErrStoreUnavailable
	}
	args := []any{observation, time.Now().UTC().Format(time.RFC3339Nano), key}
	args = append(args, s.args()...)
	r, err := d.db.ExecContext(ctx, "UPDATE browser_store_operations SET runtime_observation=?,runtime_observed_at=? WHERE operation_id=? AND "+storeWhere+" AND action='open_environment' AND phase='succeeded'", args...)
	if err != nil {
		return ErrStoreUnavailable
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return ErrStoreUnavailable
	}
	return nil
}
func (d *StoreDAO) Reports(ctx context.Context, s StoreScope, key string) ([]StoreReport, error) {
	if _, err := d.Operation(ctx, s, key); err != nil {
		return nil, err
	}
	rows, err := d.db.QueryContext(ctx, "SELECT kind,outcome,reason_code,observed_at,confirmed FROM browser_store_reports WHERE operation_id=? ORDER BY observed_at,kind", key)
	if err != nil {
		return nil, ErrStoreUnavailable
	}
	defer rows.Close()
	result := []StoreReport{}
	for rows.Next() {
		var r StoreReport
		if rows.Scan(&r.Kind, &r.Outcome, &r.ReasonCode, &r.ObservedAt, &r.Confirmed) != nil {
			return nil, ErrStoreUnavailable
		}
		result = append(result, r)
	}
	if rows.Err() != nil {
		return nil, ErrStoreUnavailable
	}
	return result, nil
}
func NewStoreReport(kind, outcome, reason string) StoreReport {
	return StoreReport{Kind: kind, Outcome: outcome, ReasonCode: reason, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
}
