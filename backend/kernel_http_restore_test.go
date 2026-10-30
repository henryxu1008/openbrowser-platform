package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestLocalHTTPLaunchCodeFailureRestoresWithdrawnOriginalBinding(t *testing.T) {
	f, source, aID := kernelHTTPFixture(t)
	create := fmt.Sprintf(`{"profile":{"profileName":"original A","coreId":%q},"autoLaunch":false}`, aID)
	status, body := f.request(t, http.MethodPost, "/api/profiles", create)
	if status != http.StatusCreated {
		t.Fatalf("create: %d / %s", status, body)
	}
	var original BrowserProfile
	for _, p := range f.app.browserMgr.Profiles {
		original = *copyBrowserProfileSnapshot(p)
	}
	bID := "20000000-0000-0000-0000-000000000002"
	source.artifact.ReleaseID = bID
	if _, err := f.app.kernels.Prepare(f.app.ctx, 7, bID, nil); err != nil {
		t.Fatal(err)
	}
	// The fixture now publishes only B: A remains prepared for its old binding.
	update := fmt.Sprintf(`{"profile":{"profileName":"staged B","coreId":%q,"userDataDir":%q},"launchCode":"invalid!","autoLaunch":false}`, bID, original.UserDataDir)
	status, body = f.request(t, http.MethodPut, "/api/profiles/"+original.ProfileId, update)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid launch code: %d / %s", status, body)
	}
	f.app.browserMgr.Mutex.Lock()
	current := copyBrowserProfileSnapshot(f.app.browserMgr.Profiles[original.ProfileId])
	f.app.browserMgr.Mutex.Unlock()
	if current.CoreId != aID || current.ProfileName != original.ProfileName || current.UserDataDir != original.UserDataDir {
		t.Fatalf("failed HTTP update left staged binding: %s / %s", current.CoreId, current.ProfileName)
	}
	stored, err := f.app.browserMgr.ProfileDAO.GetById(original.ProfileId)
	if err != nil || stored.CoreId != aID || stored.ProfileName != original.ProfileName {
		t.Fatal("failed HTTP update left persisted B")
	}
	f.assertProfiles(t, 1)
}

func TestHTTPRestoreReceiptRejectsChangedIdentityOrReplacedPostimage(t *testing.T) {
	for _, kind := range []string{"identity", "same-state-update", "replaced-object"} {
		t.Run(kind, func(t *testing.T) {
			f, source, aID := kernelHTTPFixture(t)
			original, err := f.bound.BrowserProfileCreate(BrowserProfileInput{ProfileName: "A", CoreId: aID}, 7)
			if err != nil {
				t.Fatal(err)
			}
			source.artifact.ReleaseID = "20000000-0000-0000-0000-000000000002"
			if _, err = f.app.kernels.Prepare(f.app.ctx, 7, source.artifact.ReleaseID, nil); err != nil {
				t.Fatal(err)
			}
			starter, err := f.app.scopedHTTP(7)
			if err != nil {
				t.Fatal(err)
			}
			writer := starter.(*httpProfileWriter)
			input := BrowserProfileInput{ProfileName: "B", CoreId: source.artifact.ReleaseID, UserDataDir: original.UserDataDir}
			if _, err = writer.UpdateProfile(original.ProfileId, input); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "identity":
				f.identity.replace()
			case "same-state-update":
				if _, err = f.bound.BrowserProfileUpdate(original.ProfileId, input, 7); err != nil {
					t.Fatal(err)
				}
			case "replaced-object":
				f.app.browserMgr.Mutex.Lock()
				replacement := *f.app.browserMgr.Profiles[original.ProfileId]
				f.app.browserMgr.Profiles[original.ProfileId] = &replacement
				f.app.browserMgr.Mutex.Unlock()
			}
			if err = writer.RestoreProfileUpdate(original.ProfileId); err == nil {
				t.Fatal("stale restore overwrote a new identity or postimage")
			}
			if f.app.browserMgr.Profiles[original.ProfileId].CoreId != input.CoreId {
				t.Fatal("rejected restore changed B")
			}
			if err = writer.RestoreProfileUpdate(original.ProfileId); err == nil {
				t.Fatal("restore receipt replay accepted")
			}
		})
	}
}

func TestHTTPRestoreReceiptIsOneUseAndNeverSerialized(t *testing.T) {
	f, source, aID := kernelHTTPFixture(t)
	original, err := f.bound.BrowserProfileCreate(BrowserProfileInput{ProfileName: "A", CoreId: aID}, 7)
	if err != nil {
		t.Fatal(err)
	}
	source.artifact.ReleaseID = "20000000-0000-0000-0000-000000000002"
	if _, err = f.app.kernels.Prepare(f.app.ctx, 7, source.artifact.ReleaseID, nil); err != nil {
		t.Fatal(err)
	}
	starter, err := f.app.scopedHTTP(7)
	if err != nil {
		t.Fatal(err)
	}
	writer := starter.(*httpProfileWriter)
	if _, err = writer.UpdateProfile(original.ProfileId, BrowserProfileInput{ProfileName: "B", CoreId: source.artifact.ReleaseID, UserDataDir: original.UserDataDir}); err != nil {
		t.Fatal(err)
	}
	if err = writer.RestoreProfileUpdate(original.ProfileId); err != nil {
		t.Fatal(err)
	}
	if err = writer.RestoreProfileUpdate(original.ProfileId); err == nil {
		t.Fatal("receipt restored twice")
	}
	current := f.app.browserMgr.Profiles[original.ProfileId]
	if current.CoreId != aID {
		t.Fatal("original A was not restored")
	}
	data, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "revision") {
		t.Fatal("native restore token escaped in profile JSON")
	}
}
