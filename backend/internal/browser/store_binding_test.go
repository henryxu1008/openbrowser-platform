package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"context"
	"errors"
	"github.com/google/uuid"
	"path/filepath"
	"testing"
)

func storeBindingFixture(t *testing.T) (*Manager, *StoreDAO, StoreScope) {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao, err := NewStoreDAO(db.GetConn())
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(config.DefaultConfig(), t.TempDir())
	m.ProfileDAO = NewSQLiteProfileDAO(db.GetConn())
	m.InitData()
	return m, dao, StoreScope{APIOrigin: "https://api.example.test", Issuer: "https://id.example.test", Subject: "member-user", OrganizationID: "org-a", StoreID: uuid.NewString()}
}
func newStoreBinding() StoreBinding {
	return StoreBinding{BindingID: uuid.NewString(), ProfileID: uuid.NewString(), Version: 1, BindingVersion: "1"}
}
func prepareStoreBinding(t *testing.T, d *StoreDAO, s StoreScope, b StoreBinding) string {
	t.Helper()
	key := uuid.NewString()
	if err := d.Prepare(context.Background(), s, b, key, "configure_environment"); err != nil {
		t.Fatal(err)
	}
	if err := d.Phase(context.Background(), s, key, "admitted"); err != nil {
		t.Fatal(err)
	}
	return key
}
func TestStoreBindingAtomicSaveAndImmutableOwner(t *testing.T) {
	m, d, s := storeBindingFixture(t)
	ctx := context.Background()
	b := newStoreBinding()
	key := prepareStoreBinding(t, d, s, b)
	profile, err := m.BuildStoreProfile(ProfileInput{ProfileName: "owned", ProxyConfig: ""}, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SaveStoreProfile(ctx, d, s, b, 0, key, profile, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := d.Binding(ctx, s)
	if err != nil || !found || loaded.ProfileID != b.ProfileID || loaded.Version != 1 {
		t.Fatalf("binding: %+v %v %v", loaded, found, err)
	}
	o, err := d.Operation(ctx, s, key)
	if err != nil || o.Outcome != "succeeded" {
		t.Fatalf("journal not atomic: %+v %v", o, err)
	}
	s.Subject = "another-user"
	if _, found, err = d.Binding(ctx, s); err != nil || found {
		t.Fatalf("binding moved identity: %v %v", found, err)
	}
	if _, err = d.Operation(ctx, s, key); err == nil {
		t.Fatal("operation moved identity")
	}
}
func TestStoreBindingCommitFailureRetainsIntentWithoutPublishingProfile(t *testing.T) {
	m, d, s := storeBindingFixture(t)
	ctx := context.Background()
	b := newStoreBinding()
	key := prepareStoreBinding(t, d, s, b)
	profile, err := m.BuildStoreProfile(ProfileInput{ProfileName: "cancelled"}, b)
	if err != nil {
		t.Fatal(err)
	}
	cancelled := errors.New("fixture revoked during transaction")
	checks := 0
	err = m.SaveStoreProfile(ctx, d, s, b, 0, key, profile, func() error {
		checks++
		if checks == 2 {
			return cancelled
		}
		return nil
	})
	if !errors.Is(err, cancelled) {
		t.Fatalf("wanted revocation: %v", err)
	}
	if _, found, err := d.Binding(ctx, s); err != nil || found {
		t.Fatalf("binding escaped rollback: %v %v", found, err)
	}
	var profiles int
	if err = d.db.QueryRow("SELECT count(*) FROM browser_profiles").Scan(&profiles); err != nil || profiles != 0 {
		t.Fatalf("profile escaped rollback: %d %v", profiles, err)
	}
	if len(m.Profiles) != 0 {
		t.Fatal("uncommitted profile published to original Manager")
	}
	o, err := d.Operation(ctx, s, key)
	if err != nil || o.Phase != "admitted" {
		t.Fatalf("original intent evidence erased: %+v %v", o, err)
	}
}
func TestStoreBindingDifferentFirstCandidatesShareOnePendingSlot(t *testing.T) {
	m, d, s := storeBindingFixture(t)
	ctx := context.Background()
	first := newStoreBinding()
	key := prepareStoreBinding(t, d, s, first)
	second := newStoreBinding()
	if err := d.Prepare(ctx, s, second, uuid.NewString(), "configure_environment"); !errors.Is(err, ErrStoreConflict) {
		t.Fatalf("different candidate bypassed pending slot: %v", err)
	}
	p, err := m.BuildStoreProfile(ProfileInput{ProfileName: "first"}, first)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SaveStoreProfile(ctx, d, s, first, 0, key, p, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	secondKey := prepareStoreBinding(t, d, s, second)
	p, err = m.BuildStoreProfile(ProfileInput{ProfileName: "second"}, second)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SaveStoreProfile(ctx, d, s, second, 0, secondKey, p, func() error { return nil }); !errors.Is(err, ErrStoreConflict) {
		t.Fatalf("second binding committed: %v", err)
	}
	if len(m.Profiles) != 1 {
		t.Fatal("failed first-config request polluted Manager")
	}
	var profiles int
	d.db.QueryRow("SELECT count(*) FROM browser_profiles").Scan(&profiles)
	if profiles != 1 {
		t.Fatal("failed transaction left bare profile")
	}
}
func TestStoreOperationUnknownAndAttemptAreNeverReplayed(t *testing.T) {
	_, d, s := storeBindingFixture(t)
	ctx := context.Background()
	b := newStoreBinding()
	key := prepareStoreBinding(t, d, s, b)
	if err := d.Phase(ctx, s, key, "attempted"); err != nil {
		t.Fatal(err)
	}
	if err := d.Phase(ctx, s, key, "attempted"); !errors.Is(err, ErrStoreUnknown) {
		t.Fatalf("attempt repeated: %v", err)
	}
	if err := d.Finish(ctx, s, key, "unknown", "result_unknown"); err != nil {
		t.Fatal(err)
	}
	if err := d.Prepare(ctx, s, b, uuid.NewString(), "open_environment"); !errors.Is(err, ErrStoreConflict) {
		t.Fatalf("UNKNOWN replaced by new key: %v", err)
	}
	restored, err := NewStoreDAO(d.db)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := restored.Pending(ctx, s)
	if err != nil || pending == nil || pending.OperationID != key || !pending.Attempted || pending.Phase != "unknown" {
		t.Fatalf("restart lost UNKNOWN: %+v %v", pending, err)
	}
}
