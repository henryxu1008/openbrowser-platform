package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestPublishedCoreLookupNeverFallsBackToLegacyOrImplicitDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []Core{{CoreId: "legacy", CorePath: "arbitrary-local-directory", IsDefault: true}}
	m := NewManager(cfg, t.TempDir())
	m.PublishedCores = func() []Core { return []Core{{CoreId: "published", CorePath: "prepared"}} }
	if _, ok := m.GetCore("legacy"); ok {
		t.Fatal("legacy path adopted")
	}
	if _, ok := m.GetDefaultCore(); ok {
		t.Fatal("implicit default selected")
	}
	for _, id := range []string{"", "default", "missing"} {
		if _, err := m.ResolveChromeBinary(&Profile{CoreId: id}); err == nil {
			t.Fatalf("missing binding %q fell back", id)
		}
	}
}
