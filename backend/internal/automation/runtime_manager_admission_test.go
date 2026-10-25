package automation

import (
	"ant-chrome/backend/internal/config"
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdmissionInstallAsyncReleasesFlagOnRevokedDispatch(t *testing.T) {
	for _, test := range []struct {
		name     string
		revokeAt int32
	}{
		{name: "worker_check", revokeAt: 2},
		{name: "callee_check_gap", revokeAt: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := config.DefaultConfig()
			cfg.Automation.NodeSource = config.AutomationNodeSourceBundled
			// Even an unexpected admission must not download or launch a runtime.
			cfg.Automation.RuntimeVersion = ""
			var events atomic.Int32
			manager := NewManager(root, cfg, func(string, any) { events.Add(1) }, Options{})
			var generation atomic.Uint64
			generation.Store(7)
			denied := errors.New("authentication_required")
			installing := func() bool {
				manager.mu.RLock()
				defer manager.mu.RUnlock()
				return manager.installing
			}

			// The second attempt is a new login generation, not a retry of old work.
			for attempt := 0; attempt < 2; attempt++ {
				expected := generation.Load()
				entered, release := make(chan struct{}), make(chan struct{})
				t.Cleanup(func() {
					select {
					case <-release:
					default:
						close(release)
					}
				})
				var checks atomic.Int32
				check := func() error {
					if checks.Add(1) == test.revokeAt {
						close(entered)
						<-release
					}
					if generation.Load() != expected {
						return denied
					}
					return nil
				}
				manager.InstallAsync(WithAdmission(context.Background(), check))
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatalf("attempt %d did not reach admission check %d", attempt, test.revokeAt)
				}
				if !installing() {
					t.Fatal("active async attempt did not own the install flag")
				}
				generation.Add(1)
				close(release)

				deadline := time.NewTimer(2 * time.Second)
				tick := time.NewTicker(time.Millisecond)
				for installing() {
					select {
					case <-tick.C:
					case <-deadline.C:
						tick.Stop()
						t.Fatalf("attempt %d retained installing after revocation at check %d", attempt, test.revokeAt)
					}
				}
				tick.Stop()
				deadline.Stop()
				if got := checks.Load(); got != test.revokeAt {
					t.Fatalf("revoked attempt continued: %d admission checks", got)
				}
			}
			if events.Load() != 0 {
				t.Fatal("revoked attempt reached runtime progress or failure work")
			}
			if manager.CurrentState().Installing {
				t.Fatal("fresh session still reports installation in progress")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("revoked attempt wrote runtime files")
			}
		})
	}
}

func TestAdmissionSyncInstallReleasesFlagOnValidationFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Automation.NodeSource = config.AutomationNodeSourceBundled
	cfg.Automation.RuntimeVersion = ""
	manager := NewManager(t.TempDir(), cfg, nil, Options{})
	if err := manager.EnsureInstalled(context.Background()); err == nil {
		t.Fatal("invalid runtime version was accepted")
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.installing {
		t.Fatal("synchronous install retained its flag on failure")
	}
}
