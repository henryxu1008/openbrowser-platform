package proxy

import (
	"encoding/json"
	"errors"
)

// BuildOwnedStandardProbeConfig extracts only the existing standard proxy
// mapping. No manager, pool, log, filesystem, TLS pin fetch or direct outbound.
func BuildOwnedStandardProbeConfig(raw, engine string, port int) ([]byte, error) {
	invalid := errors.New("store_proxy_check_connector_configuration_invalid")
	if port < 1 || port > 65535 {
		return nil, invalid
	}
	spec, err := parseDirectProxyBridgeSpec(raw)
	if err != nil || spec == nil {
		return nil, invalid
	}
	var cfg map[string]any
	switch engine {
	case ProxyKernelXray:
		outbound, ok, err := buildDirectProxyBridgeOutbound(raw)
		if err != nil || !ok {
			return nil, invalid
		}
		cfg = map[string]any{
			"log":       map[string]any{"loglevel": "none"},
			"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": port, "tag": "store-probe-in", "protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": false}}},
			"outbounds": []any{outbound},
			"routing":   map[string]any{"domainStrategy": "AsIs", "rules": []any{map[string]any{"type": "field", "inboundTag": []string{"store-probe-in"}, "outboundTag": "proxy-out"}}},
		}
	case ProxyKernelMihomo:
		node, err := buildMihomoNode(raw)
		if err != nil {
			return nil, invalid
		}
		node["name"] = "store-probe-upstream"
		node["udp"] = false
		cfg = map[string]any{
			"socks-port": port, "bind-address": "127.0.0.1", "allow-lan": false, "mode": "rule", "log-level": "silent", "ipv6": false, "find-process-mode": "off",
			"geo-auto-update": false, "external-controller": "", "external-ui": "", "dns": map[string]any{"enable": false},
			"profile": map[string]any{"store-selected": false, "store-fake-ip": false},
			"proxies": []any{node}, "rules": []string{"MATCH,store-probe-upstream"},
		}
	default:
		return nil, invalid
	}
	data, err := json.Marshal(cfg)
	if err != nil || len(data) > 16384 {
		return nil, invalid
	}
	return data, nil
}
