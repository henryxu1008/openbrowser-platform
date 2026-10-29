package backend

import (
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/shuomiapi"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

type contextStoreAPIFixture struct {
	storeAPI
	auth       *mockSession
	calls      int
	generation uint64
	org        string
	rejection  error
}

func (f *contextStoreAPIFixture) Context(_ context.Context, generation uint64, org string) (shuomiapi.WorkbenchContext, error) {
	f.calls++
	f.generation, f.org = generation, org
	if err := f.auth.Check(generation); err != nil {
		return shuomiapi.WorkbenchContext{}, err
	}
	if f.rejection != nil {
		return shuomiapi.WorkbenchContext{}, f.rejection
	}
	var result shuomiapi.WorkbenchContext
	result.User.ID = f.auth.Status().Subject
	result.SelectionRequired = true
	return result, nil
}

func TestStoreContextFacadeUsesNativeGenerationWithoutLocalAPIGrant(t *testing.T) {
	app, original, _ := storeAppFixture(t)
	api := &contextStoreAPIFixture{storeAPI: original, auth: original.auth}
	app.store.client = api
	app.store.generation = 0
	app.store.organization = "stale-organization"
	if _, err := original.auth.CheckAPI(); !errors.Is(err, desktopauth.ErrUnauthenticated) {
		t.Fatal("fixture unexpectedly grants the separate local API")
	}
	raw, err := NewAuthBoundApp(app).StoreWorkbenchContext(7)
	if err != nil {
		t.Fatal(err)
	}
	var result storeViewContext
	if json.Unmarshal([]byte(raw), &result) != nil || result.Context.User.ID != original.auth.Status().Subject || !result.Context.SelectionRequired {
		t.Fatal("production facade failed to publish the verified native context")
	}
	if api.calls != 1 || api.generation != 7 || api.org != "" {
		t.Fatal("native generation or prior organization was rebound")
	}
}

func TestStoreContextFacadeSeparatesLocalAdmissionFromRemoteRejection(t *testing.T) {
	for _, test := range []string{"stale-generation", "signed-out", "remote-401"} {
		t.Run(test, func(t *testing.T) {
			app, original, _ := storeAppFixture(t)
			api := &contextStoreAPIFixture{storeAPI: original, auth: original.auth}
			app.store.client = api
			generation := uint64(7)
			if test == "stale-generation" {
				generation = 6
			} else if test == "signed-out" {
				original.auth.SignOut(7)
			} else {
				api.rejection = &shuomiapi.Rejection{Code: "AUTHENTICATION_REQUIRED", StatusCode: http.StatusUnauthorized}
			}
			raw, err := NewAuthBoundApp(app).StoreWorkbenchContext(generation)
			if raw != "" || err == nil {
				t.Fatal("failed context was published")
			}
			if test == "remote-401" {
				var rejected *shuomiapi.Rejection
				if !errors.As(err, &rejected) || rejected.StatusCode != http.StatusUnauthorized || api.calls != 1 || original.auth.Check(7) != nil {
					t.Fatal("remote rejection lost its stage or changed local identity")
				}
				if err.Error() != "AUTHENTICATION_REQUIRED (store_api_response=401)" {
					t.Fatal("remote diagnostic is not bounded to protocol code and status")
				}
			} else if !errors.Is(err, desktopauth.ErrUnauthenticated) || shuomiapi.IsRejected(err) || api.calls != 0 {
				t.Fatal("local admission failure reached the remote Store API")
			}
		})
	}
}
