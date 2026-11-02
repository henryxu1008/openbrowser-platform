package storebridge

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func buildCoreFixture(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "core-fixture.exe")
	command := exec.CommandContext(ctx, "go", "build", "-o", path, "./testdata/core")
	if command.Run() != nil {
		t.Fatal("local core fixture did not build")
	}
	return path
}
func TestExecStarterUsesPrivateConfigAndPIDOwnedReadiness(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows readiness candidate")
	}
	path := buildCoreFixture(t)
	t.Setenv("CLASH_CONFIG_STRING", "fixture-secret")
	t.Setenv("XRAY_LOCATION_CONFIG", "fixture-secret")
	for _, stack := range []string{"xray", "mihomo"} {
		t.Run(stack, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			snapshot := RuntimeSnapshot{Stack: stack, XrayPath: path, MihomoPath: path}
			lease, err := New(stack, NewExecStarter(ctx, snapshot)).Acquire(ctx, fixtureProxy)
			if err != nil {
				t.Fatal("actual starter fixture readiness failed", err)
			}
			p := lease.process.(*execProcess)
			if !ownsListener(ctx, p.command.Process.Pid, p.port) || ownsListener(ctx, p.command.Process.Pid+1, p.port) {
				t.Error("readiness did not prove the owned PID")
			}
			if strings.Contains(strings.Join(p.command.Args, " ")+strings.Join(p.command.Env, " "), "fixture-secret") {
				t.Error("credential escaped private stdin")
			}
			entries, _ := os.ReadDir(p.home)
			if len(entries) != 0 {
				t.Error("private config persisted to disk")
			}
			if lease.CloseAndWait() != nil {
				t.Error("cleanup failed")
			}
			if _, err := os.Stat(p.home); !os.IsNotExist(err) {
				t.Error("owned home not removed")
			}
			if _, err := p.binary.Stat(); err == nil {
				t.Error("runtime lock not closed")
			}
			select {
			case <-p.Settled():
			default:
				t.Error("actual process cleanup not joined")
			}
		})
	}
}
func TestExecStarterIndependentLifetimeAndStartupFailures(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows readiness candidate")
	}
	path := buildCoreFixture(t)
	for _, mode := range []string{"cancel-ready", "no-read", "early-exit"} {
		t.Run(mode, func(t *testing.T) {
			copyPath := filepath.Join(t.TempDir(), mode+"-core.exe")
			data, _ := os.ReadFile(path)
			if os.WriteFile(copyPath, data, 0700) != nil {
				t.Fatal("fixture copy failed")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			starter := NewExecStarter(ctx, RuntimeSnapshot{Stack: "xray", XrayPath: copyPath})
			input, err := makeInput(fixtureProxy, "xray")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "no-read" {
				input, err = makeInput("http://fixture-user:"+strings.Repeat("p", 6000)+"@proxy.example.invalid:8080", "xray")
				if err != nil {
					t.Fatal(err)
				}
			}
			startup, stop := context.WithTimeout(ctx, 2*time.Second)
			defer stop()
			process, err := starter.Start(startup, input)
			if err != nil {
				t.Fatal(err)
			}
			p := process.(*execProcess)
			if mode == "cancel-ready" {
				if _, err = p.Ready(startup); err != nil {
					t.Fatal(err)
				}
				cancel()
			} else if mode == "no-read" {
				cancel()
				if _, err = p.Ready(startup); !errors.Is(err, context.Canceled) {
					t.Error("blocked stdin did not cancel")
				}
			} else {
				if _, err = p.Ready(startup); err == nil {
					t.Error("exited process appeared ready")
				}
			}
			select {
			case <-p.Settled():
			case <-time.After(3 * time.Second):
				_ = p.Stop()
				t.Fatal("independent cleanup did not settle")
			}
			if p.Wait() != nil {
				t.Error("process cleanup failure")
			}
			if _, err := os.Stat(p.home); !os.IsNotExist(err) {
				t.Error("failed startup left owned home")
			}
		})
	}
}
func TestExecStarterRejectsUnknownRuntimeAndForeignCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	input, _ := makeInput(fixtureProxy, "mihomo")
	if _, err := NewExecStarter(ctx, RuntimeSnapshot{AppRoot: t.TempDir(), Stack: "mihomo"}).Start(ctx, input); err != ErrRequired {
		t.Error("missing native-configured runtime admitted")
	}
	if runtime.GOOS == "windows" {
		root := t.TempDir()
		if os.Mkdir(filepath.Join(root, "bin"), 0700) != nil {
			t.Fatal("fixture folder failed")
		}
		if os.WriteFile(filepath.Join(root, "bin", "xray.exe"), []byte("untrusted fixture bytes"), 0600) != nil {
			t.Fatal("fixture file failed")
		}
		xrayInput, _ := makeInput(fixtureProxy, "xray")
		if process, err := NewExecStarter(ctx, RuntimeSnapshot{AppRoot: root, Stack: "xray"}).Start(ctx, xrayInput); process != nil || err != ErrRequired {
			t.Error("unpinned bundled runtime admitted")
		}
		if process, err := NewExecStarter(ctx, RuntimeSnapshot{Stack: "xray", MihomoPath: "fixture"}).Start(ctx, input); process != nil || err != ErrRequired {
			t.Error("cross-stack runtime admitted")
		}
	}
	foreign := t.TempDir()
	if removeOwnedHome(foreign) == nil {
		t.Fatal("foreign directory accepted for cleanup")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("foreign directory modified")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if runtime.GOOS == "windows" && ownsListener(ctx, os.Getpid()+1, listener.Addr().(*net.TCPAddr).Port) {
		t.Error("foreign PID listener accepted")
	}
}
