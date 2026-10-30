package kernelapi

import (
	"net/url"
	"strings"
	"unicode"
)

const MaxDownloadRedirects = 3

func ValidDownloadOrigins(origins []string) bool {
	if len(origins) < 1 || len(origins) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || len(origin) > 256 || u.Scheme != "https" || u.Host == "" || u.Opaque != "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "#") || u.Host != strings.ToLower(u.Host) || u.Port() == "443" || "https://"+u.Host != origin || seen[origin] {
			return false
		}
		for _, char := range u.Host {
			if char > 127 || unicode.IsSpace(char) || unicode.IsControl(char) {
				return false
			}
		}
		seen[origin] = true
	}
	return true
}

// AllowsDownloadURL uses the deployment-approved exact origins. Temporary
// signed queries are allowed only on redirects, never in persisted metadata.
func AllowsDownloadURL(raw string, origins []string, redirect bool) bool {
	limit := 2048
	if redirect {
		limit = 8192
	}
	u, err := url.Parse(raw)
	if !ValidDownloadOrigins(origins) || err != nil || len(raw) > limit || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" || strings.Contains(raw, "#") || strings.TrimSpace(raw) != raw || (!redirect && (u.RawQuery != "" || u.ForceQuery || u.Path == "" || u.Path == "/")) {
		return false
	}
	for _, origin := range origins {
		if "https://"+u.Host == origin {
			return true
		}
	}
	return false
}
