import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, create } from 'react-test-renderer'
import { KernelVersionPicker } from '../src/modules/browser/components/KernelVersionPicker'
import { acceptAuth, authSnapshot, beginAuthAction } from '../src/auth/session'

const api = vi.hoisted(() => ({ fetch: vi.fn(), prepare: vi.fn(), event: null, off: vi.fn() }))
vi.mock('../src/modules/browser/api/kernelReleases', async original => ({ ...await original(), fetchKernelReleases: api.fetch, prepareKernelRelease: api.prepare }))
vi.mock('../src/wailsjs/runtime/runtime', () => ({ EventsOn: (_, listener) => { api.event = listener; return api.off } }))
vi.mock('../src/shared/components', () => ({ Button: ({ children, variant, ...p }) => <button {...p}>{children}</button>, Select: ({ options, ...p }) => <select {...p}>{options.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}</select> }))
const release = { releaseId: '10000000-0000-0000-0000-000000000001', version: '130.0.0.1', os: 'windows', arch: 'amd64', minClientVersion: '1.0.0', maxClientVersionExclusive: '', recommended: true, installed: false }
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }
let tree, props
const choose = () => tree.root.findByType('select').props.onChange({ target: { value: release.releaseId } })
const login = () => { const epoch = beginAuthAction(); acceptAuth({ generation: authSnapshot().generation + 1, state: 'signed_in', configured: true, expiresAt: new Date(Date.now() + 60000).toISOString() }, epoch) }
beforeEach(() => { vi.clearAllMocks(); login(); api.fetch.mockResolvedValue([release]); api.prepare.mockResolvedValue(undefined); props = { value: '', scopeKey: 'store-a:1', onChange: vi.fn(), onBusyChange: vi.fn() } })
afterEach(() => { if (tree) act(() => tree.unmount()); tree = null })
async function mount() { await act(async () => { tree = create(<KernelVersionPicker {...props}/>); await Promise.resolve() }) }

it('prepares exactly one selected ID; never auto-selects a recommendation or saves configuration', async () => {
  const pending = deferred(); api.prepare.mockReturnValue(pending.promise)
  await mount(); expect(props.onChange).not.toHaveBeenCalled()
  act(() => { choose(); choose() })
  expect(api.prepare).toHaveBeenCalledExactlyOnceWith(release.releaseId)
  expect(tree.root.findByType('select').props.disabled).toBe(true)
  expect(props.onChange).not.toHaveBeenCalled()
  await act(async () => { pending.resolve(); await pending.promise })
  expect(props.onChange).toHaveBeenCalledExactlyOnceWith(release.releaseId)
  expect(api.off).toHaveBeenCalledTimes(1)
  expect(props.onBusyChange).toHaveBeenLastCalledWith(false)
})

it.each(['scope', 'value', 'auth', 'unmount'])('discards late preparation after %s replacement', async kind => {
  const pending = deferred(); api.prepare.mockReturnValue(pending.promise)
  await mount(); act(() => choose())
  act(() => {
    if (kind === 'auth') login()
    else if (kind === 'unmount') { tree.unmount(); tree = null }
    else { props = { ...props, ...(kind === 'scope' ? { scopeKey: 'store-b:2' } : { value: 'previous-binding' }) }; tree.update(<KernelVersionPicker {...props}/>) }
  })
  await act(async () => { pending.resolve(); await pending.promise })
  expect(props.onChange).not.toHaveBeenCalled()
  expect(api.off).toHaveBeenCalledTimes(1)
})

it('keeps the previous binding after preparation failure and allows retry', async () => {
  props.value = 'old-binding'; api.prepare.mockRejectedValue(new Error('kernel_artifact_integrity_failed'))
  await mount(); await act(async () => { choose(); await Promise.resolve() })
  expect(props.onChange).not.toHaveBeenCalled()
  expect(tree.root.findByType('select').props.value).toBe('old-binding')
  expect(tree.root.findByProps({ role: 'alert' }).children.join('')).toContain('校验失败')
  api.prepare.mockResolvedValue(undefined)
  await act(async () => { choose(); await Promise.resolve() })
  expect(props.onChange).toHaveBeenCalledExactlyOnceWith(release.releaseId)
})

it('can prepare a missing cache for an already selected version', async () => {
  props.value = release.releaseId
  await mount()
  const retry = tree.root.findAllByType('button').find(node => node.children.join('') === '准备此版本')
  expect(retry).toBeDefined()
  await act(async () => { retry.props.onClick(); await Promise.resolve() })
  expect(api.prepare).toHaveBeenCalledExactlyOnceWith(release.releaseId)
  expect(props.onChange).toHaveBeenCalledExactlyOnceWith(release.releaseId)
})

it('ignores foreign progress and discards an older catalog response', async () => {
  const old = deferred(), current = deferred(); api.fetch.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
  await mount()
  act(() => { props = { ...props, scopeKey: 'store-b:2' }; tree.update(<KernelVersionPicker {...props}/>) })
  await act(async () => { current.resolve([release]); await current.promise; old.resolve([]); await old.promise })
  const pending = deferred(); api.prepare.mockReturnValue(pending.promise)
  act(() => choose())
  act(() => api.event({ releaseId: release.releaseId, generation: authSnapshot().generation + 1, phase: 'ready', percent: 100 }))
  expect(tree.root.findByProps({ role: 'status' }).children.join('')).not.toContain('100%')
  await act(async () => { pending.resolve(); await pending.promise })
})
