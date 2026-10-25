package apppath

import (
	"path/filepath"
	"testing"
)

func TestNonWindowsDetachedStorageKeepsExistingResolution(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			install, state := t.TempDir(), t.TempDir()
			// Model an already-established detached installation without relying
			// on host OS permission probes or changing any real directory mode.
			key := buildCacheKey(goos, normalizeRoot(install))
			rootsCache.Store(key, roots{installRoot: install, stateRoot: state, detached: true})
			t.Cleanup(func() { rootsCache.Delete(key) })
			cases := []struct{ setting, want string }{
				{"../profiles", filepath.Join(state, "..", "profiles")},
				{"../app.db", filepath.Join(state, "..", "app.db")},
				{"profiles", filepath.Join(state, "profiles")},
				{"bin/profiles", filepath.Join(install, "bin", "profiles")},
				{".", ""},
				{filepath.Join(install, "external"), filepath.Join(install, "external")},
			}
			for _, tc := range cases {
				if got := resolveUserDataRootForOS(install, tc.setting, goos); got != tc.want {
					t.Fatalf("%s profile root changed: %s -> %s, want %s", goos, tc.setting, got, tc.want)
				}
				if got := resolveDatabasePathForOS(install, tc.setting, goos); got != tc.want {
					t.Fatalf("%s database path changed: %s -> %s, want %s", goos, tc.setting, got, tc.want)
				}
			}
		})
	}
}
