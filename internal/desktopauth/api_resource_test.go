package desktopauth

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledStoreResourceReachesOriginalPKCE(t *testing.T) {
	p := newIssuer(t)
	root := t.TempDir()
	input := `{"issuer":"` + p.issuer + `","clientId":"desktop-public","apiProjectId":"existing-api-project"}`
	if err := os.WriteFile(filepath.Join(root, PublicConfigFile), []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal("Store public resource configuration rejected", err)
	}
	m := newManager(cfg, p.server.Client(), p.clock.now)
	t.Cleanup(m.Close)
	authURL, err := m.Begin()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	want := "openid profile urn:zitadel:iam:user:resourceowner urn:zitadel:iam:org:project:id:existing-api-project:aud urn:zitadel:iam:org:project:id:zitadel:aud"
	if u.Query().Get("scope") != want {
		t.Fatal("original PKCE request lost the Store API resource contract")
	}
	if authorize(t, p, authURL) != http.StatusOK || m.Status().State != "signed_in" {
		t.Fatal("API resource scopes broke the existing Native identity exchange")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://stores.example.test/api/v1/workbench/context", nil)
	if err := m.AuthorizeRequest(m.Status().Generation, req); err != nil || req.Header.Get("Authorization") != "Bearer fixture-access-token-never-persist" {
		t.Fatal("Store API no longer uses the original in-memory access token")
	}
}

func TestBasicNativeLoginKeepsOriginalScopes(t *testing.T) {
	p := newIssuer(t)
	m := p.manager(t)
	authURL, err := m.Begin()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	if u.Query().Get("scope") != "openid profile" {
		t.Fatal("basic Native login acquired unrelated API resources")
	}
}
