package desktopauth

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

const PublicConfigFile = "oidc-public.json"

// LoadConfig reads publisher-owned public metadata beside the executable. A
// present but invalid file never falls back to inherited environment settings.
// Environment-only configuration remains available for existing developer runs.
func LoadConfig(installRoot string) (Config, error) {
	file, err := os.Open(filepath.Join(installRoot, PublicConfigFile))
	if errors.Is(err, os.ErrNotExist) {
		cfg := Config{Issuer: os.Getenv("ABFREE_OIDC_ISSUER"), ClientID: os.Getenv("ABFREE_OIDC_CLIENT_ID")}
		if !cfg.valid() {
			return Config{}, ErrConfiguration
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, ErrConfiguration
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return Config{}, ErrConfiguration
	}
	var fields map[string]string
	if strictJSON(data, &fields) != nil || len(fields) != 2 {
		return Config{}, ErrConfiguration
	}
	cfg := Config{Issuer: fields["issuer"], ClientID: fields["clientId"]}
	if !cfg.valid() {
		return Config{}, ErrConfiguration
	}
	return cfg, nil
}
