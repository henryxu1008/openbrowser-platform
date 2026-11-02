package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreProxyPublisherSourceCannotBeReplacedByInstalledFilesOrUserSettings(t *testing.T) {
	root := t.TempDir()
	spoof := []byte(`{"schemaVersion":1,"targets":[{"id":"injected","label":"Injected","url":"https://injected.example.invalid/","expectedStatus":[204]}]}`)
	if err := os.WriteFile(filepath.Join(root, "publisher_targets.json"), spoof, 0600); err != nil {
		t.Fatal(err)
	}
	a := NewApp(root)
	a.config = config.DefaultConfig()
	a.config.ProxyCheck.Targets = []config.ProxyCheckTarget{{ID: "injected", URL: "https://injected.example.invalid/", ExpectedStatus: []int{204}}}
	options := a.storeProxyOptions()
	if options.Available || len(options.Targets) != 0 || options.Reason != "TARGET_CONFIGURATION_REQUIRED" {
		t.Fatal("runtime input replaced the embedded publisher source")
	}
}
