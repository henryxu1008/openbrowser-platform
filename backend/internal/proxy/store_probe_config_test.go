package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOwnedStandardProbeConfigMatchesStackAndPreservesCredentials(t *testing.T) {
	for _, engine := range []string{ProxyKernelXray, ProxyKernelMihomo} {
		for _, scheme := range []string{"http", "https", "socks5"} {
			t.Run(engine+"/"+scheme, func(t *testing.T) {
				data, err := BuildOwnedStandardProbeConfig(scheme+"://fixture-user:fixture-secret%3A%40%2F@proxy.example.invalid:8080", engine, 31991)
				if err != nil {
					t.Fatal("standard config rejected")
				}
				var cfg map[string]json.RawMessage
				if json.Unmarshal(data, &cfg) != nil {
					t.Fatal("invalid config JSON")
				}
				if strings.Contains(string(data), "direct") || strings.Contains(string(data), "DIRECT") || strings.Contains(string(data), "allowInsecure") || strings.Contains(string(data), "skip-cert-verify") {
					t.Error("probe config adds fallback or insecure TLS")
				}
				if engine == ProxyKernelXray {
					var in []struct {
						Listen   string
						Port     int
						Settings struct{ UDP bool }
					}
					var out []struct {
						Protocol string
						Settings struct {
							Servers []struct {
								Address string
								Port    int
								Users   []struct{ User, Pass string }
							}
						}
						StreamSettings struct{ Security string }
					}
					_ = json.Unmarshal(cfg["inbounds"], &in)
					_ = json.Unmarshal(cfg["outbounds"], &out)
					if len(in) != 1 || in[0].Listen != "127.0.0.1" || in[0].Port != 31991 || in[0].Settings.UDP || len(out) != 1 || len(out[0].Settings.Servers) != 1 {
						t.Fatal("Xray scoped config drift")
					}
					server := out[0].Settings.Servers[0]
					if server.Address != "proxy.example.invalid" || server.Port != 8080 || len(server.Users) != 1 || server.Users[0].User != "fixture-user" || server.Users[0].Pass != "fixture-secret:@/" {
						t.Error("Xray credentials or destination drift")
					}
					if scheme == "https" && out[0].StreamSettings.Security != "tls" {
						t.Error("HTTPS proxy TLS missing")
					}
				} else {
					// Assert the existing mapping and documented fields without loading
					// the engine's apply/start path or adding new transitive dependencies.
					var nodes []struct {
						Server, Username, Password string
						Port                       int
						TLS, UDP                   bool
					}
					var rules []string
					_ = json.Unmarshal(cfg["proxies"], &nodes)
					_ = json.Unmarshal(cfg["rules"], &rules)
					if len(nodes) != 1 || nodes[0].Server != "proxy.example.invalid" || nodes[0].Port != 8080 || nodes[0].Username != "fixture-user" || nodes[0].Password != "fixture-secret:@/" || nodes[0].UDP || len(rules) != 1 || rules[0] != "MATCH,store-probe-upstream" {
						t.Fatal("Mihomo scoped config drift")
					}
					if scheme == "https" && !nodes[0].TLS {
						t.Error("HTTPS proxy TLS missing")
					}
				}
			})
		}
	}
}
func TestOwnedStandardProbeConfigRejectsArbitraryConfiguration(t *testing.T) {
	for _, raw := range []string{"", "direct://", "vless://fixture", "proxies: []", "http://proxy.example.invalid:8080"} {
		if _, err := BuildOwnedStandardProbeConfig(raw, "xray", 31991); err == nil {
			t.Error("non-credentialed-standard config accepted")
		}
	}
	if _, err := BuildOwnedStandardProbeConfig("http://u:p@proxy.example.invalid:8080", "other", 31991); err == nil {
		t.Error("unknown engine admitted")
	}
}
