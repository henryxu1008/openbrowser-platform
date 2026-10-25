package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthConfigLoadsPackagedIdentityBeforeBusinessInitialization(t *testing.T) {
	t.Setenv("ABFREE_OIDC_ISSUER", "")
	t.Setenv("ABFREE_OIDC_CLIENT_ID", "")
	root := t.TempDir()
	path := filepath.Join(root, "oidc-public.json")
	if err := os.WriteFile(path, []byte(`{"issuer":"https://identity.example.test","clientId":"native"}`), 0600); err != nil {
		t.Fatal(err)
	}
	a := NewApp(root)
	status := a.identity.Status()
	if !status.Configured || status.State != "signed_out" {
		t.Fatalf("native login not configured: %+v", status)
	}
	if a.businessReady {
		t.Fatal("loading identity initialized business services")
	}
	if _, err := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(err) {
		t.Fatal("loading identity wrote business data")
	}
	if err := os.WriteFile(path, []byte(`{"issuer":"http://invalid","clientId":"native"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if status := NewApp(root).identity.Status(); status.Configured || status.State != "unconfigured" {
		t.Fatalf("invalid identity admitted: %+v", status)
	}
}
