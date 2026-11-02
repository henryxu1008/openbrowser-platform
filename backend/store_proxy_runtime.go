package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/storebridge"
	"ant-chrome/backend/internal/storeproxy"
	"context"
)

func (a *App) storeProbeRuntimeSnapshot() (storebridge.RuntimeSnapshot, bool) {
	if a.config == nil {
		return storebridge.RuntimeSnapshot{}, false
	}
	return storebridge.RuntimeSnapshot{AppRoot: a.appRoot, Stack: config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType), XrayPath: a.config.Browser.XrayBinaryPath, MihomoPath: a.config.Browser.ClashBinaryPath}, true
}
func (a *App) storeNetworkProber(ctx context.Context, snapshot storebridge.RuntimeSnapshot) storeproxy.NetworkProber {
	return storeproxy.NetworkProber{Connector: storebridge.New(snapshot.Stack, storebridge.NewExecStarter(ctx, snapshot))}
}
