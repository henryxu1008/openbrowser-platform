package apppath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsCustomRelativeProfileRootMigration(t *testing.T) {
	for _, setting := range []string{"profiles", "nested/profiles", "data/profiles", "chrome/profiles", `nested\profiles`, "../external-profiles", ".", "bin/profiles", "absolute"} {
		t.Run(setting, func(t *testing.T) {
			root, state := windowsFixture(t)
			if setting == "absolute" {
				setting = filepath.Join(t.TempDir(), "profiles")
			}
			cleanSetting := filepath.FromSlash(strings.ReplaceAll(setting, `\`, "/"))
			original := filepath.Join(root, cleanSetting)
			if filepath.IsAbs(setting) {
				original = setting
			}
			want := filepath.Join(state, cleanSetting)
			if filepath.IsAbs(setting) || setting == "." || strings.HasPrefix(setting, "../") {
				want = original
			}
			if got := resolveUserDataRootForOS(root, setting, "windows"); got != want {
				t.Fatalf("profile root = %s, want %s", got, want)
			}
			writeFixture(t, original, "A/Preferences", "offline preferences")
			writeFixture(t, original, "A/Network/Cookies", "offline cookie fixture")
			writeFixture(t, root, "chrome/148/chrome.exe", "never executed")
			// Single quotes retain Windows separators. Preserve the raw config,
			// including unrelated settings, rather than saving a sanitized copy.
			raw := "browser:\n  user_data_root: '" + setting + "'\ncustom_setting: retained\n"
			writeFixture(t, root, "config.yaml", raw)
			if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
				t.Fatal(err)
			}
			assertFixture(t, original, "A/Preferences", "offline preferences")
			assertFixture(t, want, "A/Preferences", "offline preferences")
			assertFixture(t, want, "A/Network/Cookies", "offline cookie fixture")
			assertFixture(t, state, "config.yaml", raw)
			assertFixture(t, state, "chrome/148/chrome.exe", "never executed")
			writeFixture(t, want, "A/Preferences", "saved after migration")
			if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
				t.Fatal(err)
			}
			assertFixture(t, want, "A/Preferences", "saved after migration")
			if want != original {
				if err := os.Remove(filepath.Join(want, "A", "Preferences")); err != nil {
					t.Fatal(err)
				}
				if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(want, "A", "Preferences")); !os.IsNotExist(err) {
					t.Fatal("removed profile content was reimported")
				}
			}
		})
	}
}

func TestWindowsCustomProfileMigrationKeepsOwnershipAndExistingState(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "existing"}[owned], func(t *testing.T) {
			root, state := windowsFixture(t)
			windowsLegacyStateOwned = func(string) bool { return owned }
			writeFixture(t, root, "config.yaml", "browser:\n  user_data_root: profiles\n")
			writeFixture(t, root, "profiles/A/Preferences", "legacy")
			if owned {
				writeFixture(t, state, "profiles/A/Preferences", "current")
			}
			if err := ensureWritableLayoutForOS(root, "windows"); err != nil {
				t.Fatal(err)
			}
			if owned {
				assertFixture(t, state, "profiles/A/Preferences", "current")
			} else if _, err := os.Stat(filepath.Join(state, "profiles")); !os.IsNotExist(err) {
				t.Fatal("foreign custom profiles were adopted")
			}
			assertFixture(t, root, "profiles/A/Preferences", "legacy")
		})
	}
}
