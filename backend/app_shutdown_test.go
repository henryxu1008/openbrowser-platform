package backend

import (
	"ant-chrome/backend/internal/browser"
	"sync"
	"testing"
	"time"
)

func TestQuitStateTransitions(t *testing.T) {
	app := NewApp(t.TempDir())
	if app.isQuitRequested() {
		t.Fatal("new app unexpectedly has a quit request")
	}
	if app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("new authenticated host should preserve runtime services without a quit request")
	}
	legacy := &App{appState: &appState{}}
	if !legacy.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("legacy core fixture should retain full-shutdown behavior")
	}

	app.setQuitMode(quitModeAppOnly)
	if !app.isQuitRequested() {
		t.Fatal("app-only quit did not set the quit request")
	}
	if app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("app-only quit should preserve runtime services")
	}

	app.setQuitMode(quitModeFull)
	if !app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("full quit should stop runtime services")
	}
}

func TestQuitStateConcurrentAccess(t *testing.T) {
	app := NewApp(t.TempDir())
	var waitGroup sync.WaitGroup

	for i := 0; i < 8; i++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				if index%2 == 0 {
					app.setQuitMode(quitModeFull)
				} else {
					app.setQuitMode(quitModeAppOnly)
				}
				_ = app.isQuitRequested()
				_ = app.shouldStopRuntimeServicesOnShutdown()
			}
		}(i)
	}

	waitGroup.Wait()
}

func TestStopAppOnlyRuntimeServicesStopsSpeedScheduler(t *testing.T) {
	app := NewApp(t.TempDir())
	scheduler := browser.NewProxySpeedScheduler(nil, nil, time.Hour, 1)
	scheduler.Start()
	app.speedScheduler = scheduler

	app.stopAppOnlyRuntimeServices()

	if app.speedScheduler != nil {
		t.Fatal("app-only shutdown retained the speed scheduler")
	}
}
