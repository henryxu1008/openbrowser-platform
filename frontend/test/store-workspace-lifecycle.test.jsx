import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, create } from 'react-test-renderer'
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { StoreWorkspace, StoreListPage, StoreDetailPage, StoreEnvironmentEditPage } from '../src/modules/stores/StorePages'
import { BrowserEditPage } from '../src/modules/browser/pages/BrowserEditPage'
import { BrowserDetailPage } from '../src/modules/browser/pages/BrowserDetailPage'

const fixture = vi.hoisted(() => ({ api: {
  context: vi.fn(), select: vi.fn(), list: vi.fn(), store: vi.fn(), environment: vi.fn(),
  options: vi.fn(), save: vi.fn(), open: vi.fn(), close: vi.fn(), receipt: vi.fn(), audit: vi.fn(),
} }))
vi.mock('../src/modules/stores/api', async original => ({
  ...await original(), storeAPI: () => fixture.api,
}))
// Keep the production workspace, pages, adapters and React Router. View-only
// substitutes avoid DOM portals and unrelated standalone browser hooks.
vi.mock('../src/modules/browser/pages/BrowserEditPage', () => ({ BrowserEditPage: () => <section>editor</section> }))
vi.mock('../src/modules/browser/pages/BrowserDetailPage', () => ({ BrowserDetailPage: () => <section>environment</section> }))
vi.mock('../src/shared/components', () => ({
  Card: ({ children, title }) => <section>{title}{children}</section>,
  Button: ({ children, variant, ...props }) => <button {...props}>{children}</button>,
  Input: props => <input {...props}/>, Select: ({ options, ...props }) => <select {...props}/>,
  Table: ({ data, columns }) => <div>{data.map(row => <div key={row.id}>{columns.map(column =>
    <span key={column.key}>{column.render ? column.render(row[column.key], row) : row[column.key]}</span>)}</div>)}</div>,
  Modal: ({ open, children }) => open ? <section>{children}</section> : null,
  ConfirmModal: ({ open, onConfirm, onClose }) => open ? <section>
    <button onClick={onConfirm}>放弃更改并切换</button><button onClick={onClose}>继续编辑</button>
  </section> : null,
}))

const context = (org = 'org-a', revision = 1) => ({ revision, context: {
  user: { id: 'synthetic' }, effectiveOrganizationId: org, selectionRequired: false,
  organizations: ['org-a', 'org-b'].map(id => ({ id, name: id, permissions: ['workbench.store.browser.use'] })),
} })
const store = { id: 'shop-a', name: 'Shop A', platform: 'amazon', region: 'US', recordStatus: 'active' }
const environment = name => ({ binding: { bindingId: 'binding', profileId: 'profile', bindingVersion: '1' },
  name, running: false, runtimeKnown: true, pending: null, lastOperation: null, reports: [], observedAt: '' })
function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
let tree, location, navigate
function Location() { location = useLocation(); navigate = useNavigate(); return null }
async function mount(path = '/stores/shop-a') {
  await act(async () => { tree = create(<MemoryRouter initialEntries={[path]}><Location/><Routes>
    <Route path="/stores" element={<StoreWorkspace/>}>
      <Route index element={<StoreListPage/>}/>
      <Route path=":storeId" element={<StoreDetailPage/>}/>
      <Route path=":storeId/environment/edit" element={<StoreEnvironmentEditPage/>}/>
    </Route>
    <Route path="/elsewhere" element={<p>elsewhere</p>}/>
  </Routes></MemoryRouter>) })
}
const button = label => tree.root.findAllByType('button').find(node => node.children.join('') === label)
const selectOrg = id => tree.root.findByProps({ 'aria-label': '当前企业' }).props.onChange({ target: { value: id } })
const detail = () => tree.root.findByType(BrowserDetailPage).props.managed
const editor = () => tree.root.findByType(BrowserEditPage).props.managed
beforeEach(() => {
  vi.resetAllMocks()
  fixture.api.context.mockResolvedValue(context())
  fixture.api.select.mockResolvedValue(context('org-b', 2))
  fixture.api.list.mockResolvedValue({ items: [], pagination: { page: 1, pageSize: 20, total: 0 } })
  fixture.api.store.mockResolvedValue(store)
  fixture.api.environment.mockResolvedValue(environment('initial'))
  fixture.api.audit.mockResolvedValue({ items: [], nextCursor: null, coverage: [], canFilterActor: false })
  fixture.api.options.mockResolvedValue({ binding: { bindingVersion: '1' }, input: {}, cores: [] })
  fixture.api.save.mockResolvedValue(environment('saved'))
})
afterEach(() => { if (tree) act(() => tree.unmount()); tree = null })

it('keeps an initial detail deep link, then leaves it before publishing another enterprise', async () => {
  await mount('/stores/shop-a?tab=records&return=q%3Dold')
  expect(location.pathname).toBe('/stores/shop-a')
  const selected = deferred()
  fixture.api.select.mockReturnValueOnce(selected.promise)
  await act(async () => { selectOrg('org-b') })
  expect(location.pathname + location.search).toBe('/stores')
  expect(tree.root.findAllByType(BrowserDetailPage)).toHaveLength(0)
  await act(async () => { selected.resolve(context('org-b', 2)) })
  expect(fixture.api.list).toHaveBeenLastCalledWith('', '', 1, 2)
  expect(fixture.api.store).toHaveBeenCalledTimes(1)
})

it('does not reselect the current enterprise or clear its filters', async () => {
  await mount('/stores?q=kept&platform=amazon&page=2')
  await act(async () => { selectOrg('org-a') })
  expect(fixture.api.select).not.toHaveBeenCalled()
  expect(location.search).toBe('?q=kept&platform=amazon&page=2')
})

it('submits an enterprise change once even when the original handler fires twice in one event', async () => {
  await mount()
  const select = tree.root.findByProps({ 'aria-label': '当前企业' }).props.onChange, pending = deferred()
  fixture.api.select.mockReturnValueOnce(pending.promise)
  await act(async () => { select({ target: { value: 'org-b' } }); select({ target: { value: 'org-b' } }) })
  expect(fixture.api.select).toHaveBeenCalledTimes(1)
  await act(async () => { pending.resolve(context('org-b', 2)) })
  expect(location.pathname).toBe('/stores')
})

it('retains the newest explicit context lookup when earlier responses arrive late', async () => {
  fixture.api.context.mockRejectedValueOnce(new Error('unavailable'))
  await mount('/stores')
  const older = deferred(), newer = deferred(), retry = button('只读核对企业上下文').props.onClick
  fixture.api.context.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise)
  await act(async () => { retry(); retry() })
  await act(async () => { newer.resolve(context('org-b', 3)) })
  await act(async () => { older.resolve(context('org-a', 2)) })
  expect(tree.root.findByProps({ 'aria-label': '当前企业' }).props.value).toBe('org-b')
  expect(fixture.api.list).toHaveBeenCalledTimes(1)
  expect(fixture.api.list).toHaveBeenCalledWith('', '', 1, 3)
})

it('leaves old store content cleared after a failed selection and waits for an explicit context lookup', async () => {
  await mount()
  fixture.api.select.mockRejectedValueOnce(new Error('selection_unavailable'))
  await act(async () => { selectOrg('org-b') })
  expect(location.pathname).toBe('/stores')
  expect(tree.root.findAllByType(BrowserDetailPage)).toHaveLength(0)
  expect(fixture.api.context).toHaveBeenCalledTimes(1)
  expect(fixture.api.store).toHaveBeenCalledTimes(1)
  fixture.api.context.mockResolvedValueOnce(context('org-b', 2))
  await act(async () => { button('只读核对企业上下文').props.onClick() })
  expect(fixture.api.list).toHaveBeenCalledWith('', '', 1, 2)
  expect(fixture.api.select).toHaveBeenCalledTimes(1)
})

it('cancels dirty-enterprise navigation without submitting, then discards it only on confirmation', async () => {
  await mount('/stores/shop-a/environment/edit')
  await act(async () => { editor().onDirtyChange(true) })
  await act(async () => { selectOrg('org-b') })
  expect(fixture.api.select).not.toHaveBeenCalled()
  await act(async () => { button('继续编辑').props.onClick() })
  expect(location.pathname).toBe('/stores/shop-a/environment/edit')
  await act(async () => { selectOrg('org-b') })
  await act(async () => { button('放弃更改并切换').props.onClick() })
  expect(location.pathname).toBe('/stores')
  expect(fixture.api.select).toHaveBeenCalledTimes(1)
  expect(fixture.api.save).not.toHaveBeenCalled()
})

it('does not let a completed save navigate back to the old enterprise after its editor left', async () => {
  await mount('/stores/shop-a/environment/edit')
  const oldEditor = editor(), saved = deferred()
  fixture.api.save.mockReturnValueOnce(saved.promise)
  await act(async () => { await oldEditor.load() })
  const completed = oldEditor.save({ profileName: 'original' }).then(() => oldEditor.back())
  await act(async () => { selectOrg('org-b') })
  await act(async () => { saved.resolve(environment('saved')); await completed })
  expect(location.pathname).toBe('/stores')
  expect(fixture.api.store).not.toHaveBeenCalled()
  expect(fixture.api.save).toHaveBeenCalledTimes(1)
})

it('ignores editor navigation after leaving the workspace and preserves a normal save return', async () => {
  await mount('/stores/shop-a/environment/edit?return=tab%3Denvironment%26return%3Dq%253Dkept')
  const original = editor()
  await act(async () => { original.back() })
  expect(location.pathname + location.search).toBe('/stores/shop-a?tab=environment&return=q%3Dkept')
  await act(async () => { navigate('/elsewhere') })
  await act(async () => { original.back() })
  expect(location.pathname).toBe('/elsewhere')
})

it('ignores an older detail response and does not continue its environment lookup', async () => {
  await mount()
  const older = deferred(), newer = deferred()
  fixture.api.store.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise)
  const refresh = button('只读刷新').props.onClick
  await act(async () => { refresh(); refresh() })
  await act(async () => { newer.resolve({ ...store, name: 'newer' }) })
  expect(detail().name).toBe('initial')
  await act(async () => { older.resolve({ ...store, name: 'older' }) })
  expect(JSON.stringify(tree.toJSON())).toContain('newer')
  expect(JSON.stringify(tree.toJSON())).not.toContain('older')
  expect(fixture.api.environment).toHaveBeenCalledTimes(2)
})

it('does not continue a detail lookup after navigating to another store', async () => {
  const old = deferred()
  fixture.api.store.mockReturnValueOnce(old.promise)
  await mount()
  await act(async () => { navigate('/stores/shop-b') })
  await act(async () => { old.resolve(store) })
  expect(fixture.api.environment).toHaveBeenCalledTimes(1)
  expect(fixture.api.environment).toHaveBeenCalledWith('shop-b', 1)
})

it('ignores a stale environment response and a stale error without replacing the latest detail', async () => {
  await mount()
  const oldState = deferred(), oldError = deferred()
  fixture.api.environment.mockReturnValueOnce(oldState.promise).mockResolvedValue(environment('newest'))
  await act(async () => { button('只读刷新').props.onClick() })
  const refresh = button('只读刷新').props.onClick
  fixture.api.store.mockReturnValueOnce(oldError.promise).mockResolvedValue(store)
  await act(async () => { refresh(); refresh() })
  await act(async () => { oldState.resolve(environment('obsolete')); oldError.reject(new Error('obsolete error')) })
  expect(detail().name).toBe('newest')
  expect(tree.root.findAllByProps({ role: 'alert' })).toHaveLength(0)
  expect(button('只读刷新').props.disabled).toBe(false)
})

it('blocks refresh callbacks during an open action and displays its original result once', async () => {
  await mount()
  const opened = deferred(), refresh = button('只读刷新').props.onClick
  fixture.api.open.mockReturnValueOnce(opened.promise)
  await act(async () => { detail().onOpen(); refresh() })
  expect(fixture.api.store).toHaveBeenCalledTimes(1)
  expect(fixture.api.open).toHaveBeenCalledTimes(1)
  await act(async () => { opened.resolve({ ...environment('opened'), running: true }) })
  expect(detail().running).toBe(true)
})

it('blocks a stale open callback while a newer read is pending', async () => {
  await mount()
  const oldOpen = detail().onOpen, read = deferred()
  fixture.api.store.mockReturnValueOnce(read.promise)
  await act(async () => { button('只读刷新').props.onClick(); oldOpen() })
  expect(fixture.api.open).not.toHaveBeenCalled()
  expect(button('只读刷新').props.disabled).toBe(true)
  await act(async () => { read.resolve(store) })
  expect(button('只读刷新').props.disabled).toBe(false)
})
