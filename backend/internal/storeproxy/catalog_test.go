package storeproxy

import (
	"testing"
)

func fixtureTarget() Target {
	return Target{ID: "fixture", Label: "Offline target", URL: "https://target.example.invalid/check", ExpectedStatus: []int{204}}
}
func TestCatalogRejectsUnsafeTargets(t *testing.T) {
	for _, raw := range []string{"http://target.example.invalid/check", "https://user:secret@target.example.invalid/", "https://target.example.invalid/?secret=x", "https://target.example.invalid/#x", "https://127.0.0.1/", "https://127.1/", "https://localhost./", "https://target.local./", "https://target.example.invalid:444/", "file:///secret"} {
		t.Run(raw, func(t *testing.T) {
			f := fixtureTarget()
			f.URL = raw
			if _, e := NewCatalog([]Target{f}); e == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
}
func TestCatalogIsCopiedVersionedAndFailClosed(t *testing.T) {
	f := fixtureTarget()
	c, e := NewCatalog([]Target{f})
	if e != nil {
		t.Fatal(e)
	}
	f.ExpectedStatus[0] = 500
	out := c.Targets()
	out[0].ExpectedStatus[0] = 501
	target, ok := c.Resolve("fixture", c.Version())
	if !ok || target.ExpectedStatus[0] != 204 {
		t.Fatal("mutable catalog")
	}
	if _, ok = c.Resolve("fixture", "old"); ok {
		t.Fatal("stale catalog accepted")
	}
	if _, ok = c.Resolve("other", c.Version()); ok {
		t.Fatal("unknown target accepted")
	}
	empty, _ := NewCatalog(nil)
	if empty.Available() || len(empty.Targets()) != 0 {
		t.Fatal("implicit default target")
	}
	if _, e = NewCatalog([]Target{fixtureTarget(), fixtureTarget()}); e == nil {
		t.Fatal("duplicate accepted")
	}
}
