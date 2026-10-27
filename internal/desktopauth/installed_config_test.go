package desktopauth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledConfigWithoutEnvironment(t *testing.T) {
	t.Setenv("ABFREE_OIDC_ISSUER", "")
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "")
	root := t.TempDir()
	path := filepath.Join(root, PublicConfigFile)
	data := []byte(`{"issuer":"https://identity.example.test","clientId":"existing-native"}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root)
	if err != nil || cfg.Issuer != "https://identity.example.test" || cfg.ClientID != "existing-native" {
		t.Fatalf("package config: %+v, %v", cfg, err)
	}
	// A shortcut may inherit unrelated or stale environment; package identity wins.
	t.Setenv("ABFREE_OIDC_ISSUER", "https://other.example.test")
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "other")
	if again, err := LoadConfig(root); err != nil || again != cfg {
		t.Fatalf("inherited identity replaced package: %+v, %v", again, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(data) {
		t.Fatal("public package file changed")
	}
}

func TestInstalledConfigRejectsInvalidFileWithoutFallback(t *testing.T) {
	t.Setenv("ABFREE_OIDC_ISSUER", "https://valid-env.example.test")
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "env-native")
	for name, input := range map[string]string{
		"missing client":    `{"issuer":"https://identity.example.test"}`,
		"duplicate":         `{"issuer":"https://identity.example.test","issuer":"https://other.example.test","clientId":"native"}`,
		"unknown secret":    `{"issuer":"https://identity.example.test","clientId":"native","clientSecret":"fixture"}`,
		"empty API project": `{"issuer":"https://identity.example.test","clientId":"native","apiProjectId":""}`,
		"null API project":  `{"issuer":"https://identity.example.test","clientId":"native","apiProjectId":null}`,
		"scope injection":   `{"issuer":"https://identity.example.test","clientId":"native","apiProjectId":"project offline_access"}`,
		"URN injection":     `{"issuer":"https://identity.example.test","clientId":"native","apiProjectId":"project:aud"}`,
		"extra scopes":      `{"issuer":"https://identity.example.test","clientId":"native","apiProjectId":"project","scopes":"openid"}`,
		"alias":             `{"Issuer":"https://identity.example.test","clientId":"native"}`,
		"http":              `{"issuer":"http://identity.example.test","clientId":"native"}`,
		"userinfo":          `{"issuer":"https://fixture@identity.example.test","clientId":"native"}`,
		"null":              `{"issuer":"https://identity.example.test","clientId":null}`,
		"non-string":        `{"issuer":"https://identity.example.test","clientId":123}`,
		"trailing":          `{"issuer":"https://identity.example.test","clientId":"native"}{}`,
		"large":             strings.Repeat(" ", 4097),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, PublicConfigFile), []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(root)
			if !errors.Is(err, ErrConfiguration) || cfg != (Config{}) {
				t.Fatalf("invalid package fell back: %+v %v", cfg, err)
			}
		})
	}
}

func TestInstalledConfigPreservesEnvironmentOnlyDeveloperRuns(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ABFREE_OIDC_ISSUER", "https://localhost:35443")
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "existing-native")
	if cfg, err := LoadConfig(root); err != nil || cfg.ClientID != "existing-native" {
		t.Fatalf("legacy developer input: %+v %v", cfg, err)
	}
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "")
	if _, err := LoadConfig(root); !errors.Is(err, ErrConfiguration) {
		t.Fatal("incomplete input admitted")
	}
	if err := os.Mkdir(filepath.Join(root, PublicConfigFile), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "existing-native")
	if _, err := LoadConfig(root); !errors.Is(err, ErrConfiguration) {
		t.Fatal("unreadable package fell back to env")
	}
}
