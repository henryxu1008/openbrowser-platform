package backend

import (
	"ant-chrome/backend/internal/browser"
	"reflect"
)

// Owned by one scoped HTTP writer, populated only by a successful checked
// native update. Neither snapshots nor recovery inputs come from the caller.
type httpProfileUpdateReceipt struct {
	before, after *BrowserProfile
	object        *BrowserProfile
	used          bool
}

func (a *httpProfileWriter) RestoreProfileUpdate(id string) error {
	r := a.receipt
	if r == nil || r.used || r.before == nil || r.after == nil || r.before.ProfileId != id || r.after.ProfileId != id {
		return errPlatformKernelOnly
	}
	r.used = true
	check := a.captureAdmission()
	input := browser.ProfileInput{ProfileName: r.before.ProfileName, UserDataDir: r.before.UserDataDir, CoreId: r.before.CoreId, RestoreLastSession: r.before.RestoreLastSession, FingerprintArgs: append([]string{}, r.before.FingerprintArgs...), ProxyId: r.before.ProxyId, ProxyConfig: r.before.ProxyConfig, MemoryLimitMB: r.before.MemoryLimitMB, LaunchArgs: append([]string{}, r.before.LaunchArgs...), Tags: append([]string{}, r.before.Tags...), Keywords: append([]string{}, r.before.Keywords...), GroupId: r.before.GroupId}
	_, err := a.browserMgr.UpdateChecked(id, input, func() error {
		if err := check(); err != nil {
			return err
		}
		current := a.browserMgr.Profiles[id]
		if current != r.object || !reflect.DeepEqual(copyBrowserProfileSnapshot(current), r.after) {
			return errPlatformKernelOnly
		}
		return nil
	})
	return err
}
