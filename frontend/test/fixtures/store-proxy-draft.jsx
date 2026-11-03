// Production shell/editor/Store adapter, synthetic responses only. All mutations
// and network checks fail closed. Counts contain method names, never payloads.
import React from 'react'
import { createRoot } from 'react-dom/client'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { AuthGate } from '../../src/auth/AuthGate'
import { acceptAuth, authSnapshot, beginAuthAction } from '../../src/auth/session'
import { ThemeProvider } from '../../src/shared/theme'
import { ShellSlotsProvider } from '../../src/shared/layout/ShellSlots'
import { BusinessShell } from '../../src/App'
import '../../src/index.css'

const params = new URLSearchParams(location.search), counts = {}, unexpected = []
const kernelFailure = { retry: 'kernel_preparation_failed', forbidden: 'kernel_release_forbidden' }[params.get('kernel')]
const status = { generation: authSnapshot().generation + 1, state: 'signed_in', configured: true, subject: '离线示例用户', expiresAt: new Date(Date.now() + 3600000).toISOString() }
const release = { releaseId: '10000000-0000-0000-0000-000000000001', version: '130.0.0.1', os: 'windows', arch: 'amd64', recommended: true, installed: !kernelFailure }
let organization = 'fixture-org-a', revision = 7
const json = value => Promise.resolve(JSON.stringify(value))
const store = { id: 'fixture-store', name: '示例店铺', platform: 'temu', region: 'US', recordStatus: 'active' }
const context = () => ({ revision, context: { user: { id: 'synthetic' }, effectiveOrganizationId: organization, selectionRequired: false, organizations: ['fixture-org-a', 'fixture-org-b'].map((id, i) => ({ id, name: `示例企业 ${i + 1}`, permissions: ['workbench.store.browser.use'] })) } })
const environment = { binding: { bindingId: 'fixture-binding', profileId: 'fixture-profile', bindingVersion: '4' }, name: store.name, coreId: release.releaseId, proxySummary: 'SOCKS5 · 未检测', running: false, runtimeKnown: true, observedAt: '', pending: null, lastOperation: null }
const input = { profileName: store.name, userDataDir: '', coreId: release.releaseId, restoreLastSession: 'enabled', fingerprintArgs: [], proxyId: '', proxyConfig: 'socks5://proxy.example.invalid:1080', memoryLimitMb: 512, launchArgs: [], tags: [], keywords: [], groupId: '' }
const native = {
  AuthStatus: () => Promise.resolve(status), GetAppConfig: () => Promise.resolve({ version: 'offline-fixture' }), FrontendOperationLog: () => Promise.resolve(),
  StoreWorkbenchContext: () => json(context()), StoreWorkbenchSelectOrganization: id => { organization = id; revision++; return json(context()) },
  StoreWorkbenchList: () => json({ items: [store], pagination: { page: 1, pageSize: 20, total: 1 } }), StoreWorkbenchGet: () => json(store), StoreEnvironmentGet: () => json(environment),
  StoreEnvironmentOptions: () => params.get('deny') === '1' ? Promise.reject(new Error('PERMISSION_DENIED')) : json({ input, cores: [], binding: environment.binding, proxyCheck: { available: false, reason: 'TARGET_CONFIGURATION_REQUIRED', catalogVersion: '', targets: [] } }),
  BrowserKernelCatalog: () => json([release]),
}
window.proxyDraftFixture = { counts, unexpected }
if (kernelFailure) {
  input.coreId = '10000000-0000-0000-0000-000000000002'
  native.BrowserKernelPrepare = () => {
    if (kernelFailure === 'kernel_release_forbidden' || counts.BrowserKernelPrepare === 1) return Promise.reject(new Error(kernelFailure))
    release.installed = true
    return Promise.resolve()
  }
}
window.go = { main: { App: new Proxy(native, { get: (target, key) => { const method = target[key]; return (...args) => {
  counts[key] = (counts[key] || 0) + 1
  if (!(key in target)) { unexpected.push(String(key)); return Promise.reject(new Error('Offline fixture rejects this native method')) }
  return method(...args)
} } }) } }
window.runtime = { EventsOnMultiple: () => () => {} }
window.fetch = () => Promise.reject(new Error('Offline fixture rejects fetch'))
acceptAuth(status, beginAuthAction())
function Trace() { const route = useLocation(); return <p style={{ position: 'fixed', right: 0, bottom: 0, zIndex: 10000, background: '#fff', color: '#475569', padding: 4, fontSize: 11 }}>离线合成夹具 · {route.pathname}</p> }
createRoot(document.getElementById('root')).render(<ThemeProvider defaultTheme={params.get('theme') === 'dark' ? 'dark' : 'light'}><MemoryRouter initialEntries={['/stores/fixture-store/environment/edit']}><ShellSlotsProvider><AuthGate><BusinessShell /></AuthGate></ShellSlotsProvider><Trace /></MemoryRouter></ThemeProvider>)
