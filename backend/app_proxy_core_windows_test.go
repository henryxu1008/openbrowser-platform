package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowsInstalledProxyDownloadsUseUserStateAndKeepBundledRuntimes(t *testing.T) {
	base := t.TempDir()
	programs, local := filepath.Join(base, "Program Files"), filepath.Join(base, "Local")
	root := filepath.Join(programs, "Ant Browser")
	t.Setenv("ProgramFiles", programs)
	t.Setenv("LOCALAPPDATA", local)
	a := &App{appState: &appState{appRoot: root, config: config.DefaultConfig()}}
	target := proxyCoreTarget{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	connector := a.config.Browser.DefaultConnectorType
	for _, name := range []string{"xray", "sing-box", "mihomo"} {
		t.Run(name, func(t *testing.T) {
			spec, err := normalizeProxyCoreSpec(name)
			if err != nil {
				t.Fatal(err)
			}
			binary := proxyCoreBinaryName(spec.BinaryBase, target.GOOS)
			bundled := filepath.Join(root, "bin", binary)
			if err := os.MkdirAll(filepath.Dir(bundled), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(bundled, []byte("bundled fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if path, _, ok := findInstalledProxyCoreBinary(root, spec, target); !ok || path != bundled {
				t.Fatalf("bundled runtime unavailable: %s", path)
			}
			install := proxyCoreInstallDir(a, spec, target)
			want := filepath.Join(local, "ant-browser", "bin", target.GOOS+"-"+target.GOARCH, spec.Core)
			if install != want {
				t.Fatalf("download would write into installation: %s", install)
			}
			if err := os.MkdirAll(install, 0755); err != nil {
				t.Fatal(err)
			}
			downloaded := filepath.Join(install, binary)
			if err := os.WriteFile(downloaded, []byte("downloaded fixture, never executed"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := a.saveProxyCoreBinaryPath(spec, downloaded); err != nil {
				t.Fatal(err)
			}
			if status := a.proxyCoreStatus(spec, target); !status.Configured || status.BinaryPath != downloaded {
				t.Fatalf("downloaded runtime not reused: %+v", status)
			}
			if a.config.Browser.DefaultConnectorType != connector {
				t.Fatal("download changed the selected connector stack")
			}
			if data, err := os.ReadFile(bundled); err != nil || string(data) != "bundled fixture" {
				t.Fatal("bundled runtime changed")
			}
		})
	}
}
