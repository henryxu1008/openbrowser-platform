package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWailsLoggerOmitsIPCPayloadsAndKeepsOrdinaryEvents(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	log := NewWailsLogger(root)
	for _, message := range []string{`process message error: C{"name":"main.App.StoreProxyCheck","args":["http://fixture-user:fixture-secret@host:8080"]} -> invalid`,
		`process message error: c{"id":123,"args":["fixture-secret"]} -> invalid`,
		`json call result data: {"proxyConfig":"fixture-secret"}`, `Unknown message returned from dispatcher: fixture-secret`,
		`main.App.StoreProxyCheck fixture-secret`, `store-proxy-check-v1 fixture-secret`} {
		log.Error(message)
	}
	log.Info("ordinary lifecycle event")
	data, e := os.ReadFile(filepath.Join(root, "data", "logs", "app-lifecycle.log"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), "fixture-secret") || strings.Contains(string(data), "fixture-user") || !strings.Contains(string(data), "payload omitted") || !strings.Contains(string(data), "ordinary lifecycle event") {
		t.Fatal("IPC payload not omitted or ordinary log lost")
	}
}
