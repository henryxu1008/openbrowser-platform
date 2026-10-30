// Package apiconfig reads the publisher-owned API origin shared by native API clients.
package apiconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var ErrConfiguration = errors.New("api_service_configuration_required")

type Config struct {
	APIOrigin string `json:"apiOrigin"`
}

func Load(root string) (Config, error) {
	file, err := os.Open(filepath.Join(root, "shuomi-public.json"))
	if err != nil {
		return Config{}, ErrConfiguration
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return Config{}, ErrConfiguration
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') || !dec.More() {
		return Config{}, ErrConfiguration
	}
	name, err := dec.Token()
	if err != nil || name != "apiOrigin" {
		return Config{}, ErrConfiguration
	}
	var value string
	if dec.Decode(&value) != nil || dec.More() {
		return Config{}, ErrConfiguration
	}
	token, err = dec.Token()
	if err != nil || token != json.Delim('}') || dec.Decode(new(any)) != io.EOF {
		return Config{}, ErrConfiguration
	}
	cfg := Config{APIOrigin: value}
	if !cfg.Valid() {
		return Config{}, ErrConfiguration
	}
	return cfg, nil
}

func (c Config) Valid() bool {
	u, err := url.Parse(c.APIOrigin)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawPath == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.Opaque == "" && strings.TrimSpace(c.APIOrigin) == c.APIOrigin && !strings.ContainsAny(c.APIOrigin, "?#\\")
}
