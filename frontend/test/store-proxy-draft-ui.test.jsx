import { afterEach, expect, it, vi } from 'vitest'
import { act, create } from 'react-test-renderer'
import { StoreProxyDraft } from '../src/modules/stores/StoreProxyDraft'
vi.mock('../src/shared/components', () => ({
  Button: ({ children, ...props }) => <button {...props}>{children}</button>,
  FormItem: ({ children }) => <div>{children}</div>, Input: props => <input {...props}/>, Select: props => <select {...props}/>,
  Modal: ({ children, footer, onClose }) => <div role="dialog"><button onClick={onClose}>close</button>{children}{footer}</div>,
}))
let tree
afterEach(() => { if (tree) act(() => tree.unmount()); tree = null })
const button = label => tree.root.findAllByType('button').find(node => node.children.join('') === label)
const field = label => tree.root.findByProps({ 'aria-label': label })
const submit = () => tree.root.findByType('form').props.onSubmit
function fill() {
  act(() => button('填写代理参数').props.onClick())
  for (const [label, value] of Object.entries({ 代理地址: 'proxy.example.invalid', 代理端口: '8080', 代理账号: 'fixture', 代理密码: 'synthetic-secret' })) act(() => field(label).props.onChange({ target: { value } }))
}
it('rejects the old context callback even when the component itself is retained', () => {
  const apply = vi.fn(() => true), contextA = {}, contextB = {}, raw = 'http://old.example.invalid:8080'
  act(() => { tree = create(<StoreProxyDraft context={contextA} proxyConfig={raw} disabled={false} onApply={apply}/>) })
  const oldOpen = button('填写代理参数').props.onClick
  fill(); const oldSubmit = submit()
  act(() => { tree.update(<StoreProxyDraft context={contextB} proxyConfig={raw} disabled={false} onApply={apply}/>); })
  act(() => { oldOpen(); oldSubmit({ preventDefault() {} }) })
  expect(apply).not.toHaveBeenCalled(); expect(tree.root.findAllByType('form')).toHaveLength(0)
  act(() => button('填写代理参数').props.onClick()); expect(field('代理密码').props.value).toBe('')
})
it('clears credentials on permission/readiness loss and permits only a fresh form after recovery', () => {
  const context = {}, apply = vi.fn(() => true), raw = 'http://old.example.invalid:8080'
  act(() => { tree = create(<StoreProxyDraft context={context} proxyConfig={raw} disabled={false} onApply={apply}/>) })
  fill(); const oldSubmit = submit()
  act(() => tree.update(<StoreProxyDraft context={context} proxyConfig={raw} disabled onApply={apply}/>))
  act(() => oldSubmit({ preventDefault() {} })); expect(apply).not.toHaveBeenCalled()
  act(() => tree.update(<StoreProxyDraft context={context} proxyConfig={raw} disabled={false} onApply={apply}/>))
  act(() => button('填写代理参数').props.onClick()); expect(field('代理密码').props.value).toBe('')
  act(() => oldSubmit({ preventDefault() {} })); expect(apply).not.toHaveBeenCalled()
})
