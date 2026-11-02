// Package storeproxy contains the operation-owned Store probe primitives. It has
// no profile/pool/connector managers, loggers or persistence capability.
package storeproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var ErrCatalog = errors.New("store_proxy_check_catalog_invalid")
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var hostPattern = regexp.MustCompile(`(?i)^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,62}$`)

type Target struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	URL            string `json:"url"`
	ExpectedStatus []int  `json:"expectedStatus"`
}
type Catalog struct {
	targets map[string]Target
	order   []string
	version string
}

// NewCatalog validates an already publisher-owned source. This validator does
// not make user settings trustworthy. Production currently supplies no targets.
func NewCatalog(targets []Target) (Catalog, error) {
	c := Catalog{targets: map[string]Target{}}
	if len(targets) > 8 {
		return Catalog{}, ErrCatalog
	}
	for _, t := range targets {
		u, e := url.Parse(t.URL)
		host := ""
		if u != nil {
			host = strings.TrimSuffix(u.Hostname(), ".")
		}
		if !idPattern.MatchString(t.ID) || t.Label == "" || len(t.Label) > 120 || strings.TrimSpace(t.Label) != t.Label || strings.ContainsAny(t.Label, "\x00\r\n\t") || len(t.URL) > 2048 || e != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(t.URL, "#") || u.Opaque != "" || u.Host == "" || net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.HasSuffix(strings.ToLower(host), ".localhost") || strings.HasSuffix(strings.ToLower(host), ".local") || (u.Port() != "" && u.Port() != "443") || len(t.ExpectedStatus) == 0 || len(t.ExpectedStatus) > 4 {
			return Catalog{}, ErrCatalog
		}
		if !hostPattern.MatchString(host) || len(host) > 253 {
			return Catalog{}, ErrCatalog
		}
		if _, ok := c.targets[t.ID]; ok {
			return Catalog{}, ErrCatalog
		}
		for _, status := range t.ExpectedStatus {
			if status < 200 || status > 299 {
				return Catalog{}, ErrCatalog
			}
		}
		t.ExpectedStatus = append([]int(nil), t.ExpectedStatus...)
		c.targets[t.ID] = t
		c.order = append(c.order, t.ID)
	}
	data, _ := json.Marshal(targets)
	sum := sha256.Sum256(data)
	c.version = hex.EncodeToString(sum[:])
	return c, nil
}
func (c Catalog) Version() string { return c.version }
func (c Catalog) Targets() []Target {
	out := []Target{}
	for _, id := range c.order {
		t := c.targets[id]
		t.ExpectedStatus = append([]int(nil), t.ExpectedStatus...)
		out = append(out, t)
	}
	return out
}
func (c Catalog) Resolve(id, version string) (Target, bool) {
	if version == "" || version != c.version {
		return Target{}, false
	}
	t, ok := c.targets[id]
	t.ExpectedStatus = append([]int(nil), t.ExpectedStatus...)
	return t, ok
}
func (c Catalog) Available() bool { return len(c.targets) > 0 }
