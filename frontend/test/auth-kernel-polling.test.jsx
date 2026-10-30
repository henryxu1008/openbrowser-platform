import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, create } from 'react-test-renderer'
import { AuthGate } from '../src/auth/AuthGate'
import { acceptAuth, authEpoch, authSnapshot, beginAuthAction } from '../src/auth/session'
import { KernelVersionPicker } from '../src/modules/browser/components/KernelVersionPicker'

const native = vi.hoisted(() => ({ AuthStatus: vi.fn(), AuthLogin: vi.fn(), AuthLogout: vi.fn(), AuthCancel: vi.fn(), AuthGrantAPI: vi.fn(), QuitAppOnly: vi.fn() }))
const api = vi.hoisted(() => ({ fetch: vi.fn(), prepare: vi.fn(), off: vi.fn() }))
vi.mock('../src/wailsjs/go/main/App', () => native)
vi.mock('../src/modules/browser/api/kernelReleases', async original => ({ ...await original(), fetchKernelReleases: api.fetch, prepareKernelRelease: api.prepare }))
vi.mock('../src/wailsjs/runtime/runtime', () => ({ EventsOn: () => api.off }))
vi.mock('../src/shared/components', () => ({
  Button: ({ children, variant, size, loading, ...props }) => <button {...props}>{children}</button>,
  Select: ({ options, ...props }) => <select {...props}>{options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select>,
  Loading: ({ text }) => <span>{text}</span>,
}))
const release = { releaseId: '10000000-0000-0000-0000-000000000001', version: '130.0.0.1', installed: false }
const deferred = () => { let resolve; const promise = new Promise(yes => { resolve = yes }); return { promise, resolve } }
let tree, current, onChange
beforeEach(() => {
  vi.useFakeTimers(); vi.clearAllMocks(); globalThis.window = { setInterval, clearInterval }
  current = { generation: authSnapshot().generation + 1, state: 'signed_in', configured: true, issuer: 'https://synthetic.invalid', subject: 'synthetic', expiresAt: new Date(Date.now() + 60000).toISOString(), apiGranted: false }
  acceptAuth(current, beginAuthAction()); native.AuthStatus.mockImplementation(async () => ({ ...current }))
  api.fetch.mockResolvedValue([release]); api.prepare.mockResolvedValue(undefined); onChange = vi.fn()
})
afterEach(() => { if (tree) act(() => tree.unmount()); tree = null; delete globalThis.window; vi.useRealTimers() })
async function mount() { await act(async () => { tree = create(<AuthGate><KernelVersionPicker value="" scopeKey="catalog" onChange={onChange}/></AuthGate>) }) }
async function poll(ms = 3000) { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
const button = label => tree.root.findAllByType('button').find(node => node.children.join('') === label)
const options = () => tree.root.findAllByType('option').map(node => node.props.value)
const boundaries = ['generation', 'subject', 'issuer', 'epoch', 'API admission', 'unknown', 'logout']
function invalidate(kind) {
  if (kind === 'epoch') beginAuthAction()
  else {
    const change = { generation: { generation: current.generation + 1 }, subject: { subject: 'replacement' }, issuer: { issuer: 'https://replacement.invalid' }, 'API admission': { apiGranted: true }, unknown: { state: 'unknown' }, logout: { state: 'signed_out' } }[kind]
    acceptAuth({ ...current, ...change }, authEpoch())
  }
}

it('lets a catalog response complete across repeated real AuthGate polls', async () => {
  const pending = deferred(); api.fetch.mockReturnValueOnce(pending.promise)
  await mount(); await poll(); expect(native.AuthStatus.mock.calls.length).toBeGreaterThan(2)
  await act(async () => { pending.resolve([release]); await pending.promise })
  expect(options()).toContain(release.releaseId); expect(tree.root.findAllByProps({ role: 'alert' })).toHaveLength(0)
  expect(api.prepare).not.toHaveBeenCalled()
})

it('keeps the loaded catalog stable and allows a manual refresh to finish across polls', async () => {
  await mount(); await poll(); expect(options()).toContain(release.releaseId)
  const pending = deferred(), updated = { ...release, releaseId: '10000000-0000-0000-0000-000000000002' }
  api.fetch.mockReturnValueOnce(pending.promise); act(() => button('刷新版本').props.onClick())
  await poll(); await act(async () => { pending.resolve([updated]); await pending.promise })
  expect(options()).toContain(updated.releaseId); expect(options()).not.toContain(release.releaseId)
  expect(tree.root.findByType('select').props.disabled).toBe(false)
  expect(tree.root.findAllByProps({ role: 'alert' })).toHaveLength(0); expect(onChange).not.toHaveBeenCalled()
})

it('does not cancel a synthetic preparation on equivalent polls', async () => {
  const pending = deferred(); api.prepare.mockReturnValueOnce(pending.promise)
  await mount(); act(() => tree.root.findByType('select').props.onChange({ target: { value: release.releaseId } }))
  await poll(); expect(tree.root.findByType('select').props.disabled).toBe(true)
  await act(async () => { pending.resolve(); await pending.promise })
  expect(onChange).toHaveBeenCalledExactlyOnceWith(release.releaseId); expect(api.prepare).toHaveBeenCalledTimes(1)
})

it('unmounts business UI and rejects late synthetic preparation at clock expiry with an identical native status', async () => {
  const pending = deferred(); api.prepare.mockReturnValueOnce(pending.promise)
  await mount(); act(() => tree.root.findByType('select').props.onChange({ target: { value: release.releaseId } }))
  await poll(60000); expect(tree.root.findAllByType('select')).toHaveLength(0)
  await act(async () => { pending.resolve(); await pending.promise }); expect(onChange).not.toHaveBeenCalled()
})

it.each(boundaries)('discards a late catalog response after a real %s boundary', async kind => {
  const pending = deferred(); api.fetch.mockReturnValueOnce(pending.promise).mockResolvedValue([])
  await mount(); act(() => invalidate(kind))
  await act(async () => { pending.resolve([release]); await pending.promise })
  expect(options()).not.toContain(release.releaseId); expect(api.prepare).not.toHaveBeenCalled()
})

it.each(boundaries)('discards late synthetic preparation after a real %s boundary', async kind => {
  const pending = deferred(); api.prepare.mockReturnValueOnce(pending.promise)
  await mount(); act(() => tree.root.findByType('select').props.onChange({ target: { value: release.releaseId } }))
  act(() => invalidate(kind))
  expect(options()).not.toContain(release.releaseId)
  await act(async () => { pending.resolve(); await pending.promise }); expect(onChange).not.toHaveBeenCalled()
})
