import { afterEach, expect, it, vi } from 'vitest'
import { act, create } from 'react-test-renderer'
import { MemoryRouter } from 'react-router-dom'
import { StoreProxyCheck } from '../src/modules/stores/StoreProxyCheck'

vi.mock('../src/modules/browser/components/KernelVersionPicker',()=>({KernelVersionPicker:()=>null}))
vi.mock('../src/modules/browser/components/FingerprintPanel',()=>({FingerprintPanel:()=>null}))
vi.mock('../src/modules/browser/components/ProxyPickerModal',()=>({ProxyPickerModal:()=>null}))
const scope={identity:'member',generation:1,organization:'org',revision:3,store:'shop',bindingVersion:'1',connector:'xray'}
const targets=[{id:'fixture-target',label:'离线示例目标',url:'https://target.example.invalid/check'}]
const proxyConfig='socks5://proxy.example.invalid:1080'
const ok=request=>({schema:'store-proxy-check-v1',requestId:request.requestId,targetId:request.targetId,syntax:'passed',tcp:'passed',target:'passed',reason:'NONE'})
const deferred=()=>{let resolve;const promise=new Promise(r=>{resolve=r});return {promise,resolve}}
let tree
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null;vi.unstubAllGlobals()})
const button=text=>tree.root.findAllByType('button').find(n=>n.children.join('')===text)
const select=()=>tree.root.findAllByType('select').find(n=>n.props['aria-label']==='检查目标')
async function mount(port,extra={}){await act(async()=>{tree=create(<StoreProxyCheck scope={scope} proxyConfig={proxyConfig} port={port} {...extra}/>)})}
async function choose(){await act(async()=>select().props.onChange({target:{value:'fixture-target'}}))}

it('requires deliberate selection, uses secondary action, and never starts on mount',async()=>{
  const check=vi.fn(async r=>ok(r));await mount({targets,check})
  expect(button('检查此代理').props.disabled).toBe(true);expect(check).not.toHaveBeenCalled()
  await choose();expect(button('检查此代理').props.disabled).toBe(false)
  await act(async()=>button('检查此代理').props.onClick())
  expect(check).toHaveBeenCalledTimes(1);expect(JSON.stringify(tree.toJSON())).toContain('代理服务器 TCP')
  expect(JSON.stringify(tree.toJSON())).toContain('结果不证明地区正确或商家账号可登录')
})
it('shows positive TCP evidence with failed target separately and never raw errors',async()=>{
  const check=vi.fn(async r=>({...ok(r),target:'failed',reason:'TARGET_UNREACHABLE',error:'fixture-secret',recentLogs:['fixture-secret']}))
  await mount({targets,check});await choose();await act(async()=>button('检查此代理').props.onClick())
  expect(JSON.stringify(tree.toJSON())).toContain('未能经代理访问所选目标')
  expect(JSON.stringify(tree.toJSON())).not.toContain('fixture-secret')
})
it('changes scope during a pending check and ignores the old completion',async()=>{
  const p=deferred(),check=vi.fn(()=>p.promise),port={targets,check};await mount(port);await choose()
  await act(async()=>button('检查此代理').props.onClick())
  await act(async()=>tree.update(<StoreProxyCheck scope={{...scope,revision:4}} proxyConfig={proxyConfig} port={port}/>))
  expect(check.mock.calls[0][1].aborted).toBe(true)
  await act(async()=>p.resolve(ok(check.mock.calls[0][0])))
  expect(JSON.stringify(tree.toJSON())).not.toContain('代理服务器 TCP')
})
it('retains cancel status and disables retries until the mock worker actually settles',async()=>{
  const p=deferred(),check=vi.fn(()=>p.promise);await mount({targets,check});await choose()
  await act(async()=>button('检查此代理').props.onClick());await act(async()=>button('取消检查').props.onClick())
  expect(button('检查此代理').props.disabled).toBe(true)
  expect(JSON.stringify(tree.toJSON())).toContain('等待检查停止后可重试')
  await act(async()=>p.resolve(ok(check.mock.calls[0][0])))
  expect(button('检查此代理').props.disabled).toBe(false)
  expect(JSON.stringify(tree.toJSON())).not.toContain('代理服务器 TCP')
})
it('aborts when unmounted and rejects a captured stale action',async()=>{
  const p=deferred(),check=vi.fn(()=>p.promise);await mount({targets,check});await choose()
  const action=button('检查此代理').props.onClick;await act(async()=>action())
  act(()=>tree.unmount());tree=null;expect(check.mock.calls[0][1].aborted).toBe(true)
  await act(async()=>{p.resolve(ok(check.mock.calls[0][0]));action()});expect(check).toHaveBeenCalledTimes(1)
})
it('mounts through the actual managed editor with no save, back or native transport side effect',async()=>{
  const native=vi.fn(()=>{throw new Error('Unexpected native transport')})
  vi.stubGlobal('window',{setTimeout,clearTimeout,go:{backend:{AuthBoundApp:new Proxy({},{get:()=>native})}}})
  vi.stubGlobal('document',{body:{style:{}},addEventListener:vi.fn(),removeEventListener:vi.fn()})
  const {BrowserEditPage}=await import('../src/modules/browser/pages/BrowserEditPage')
  const check=vi.fn(async r=>ok(r)),save=vi.fn(),back=vi.fn()
  const input={profileName:'示例店铺',userDataDir:'',coreId:'',restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig,memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
  const managed={title:'店铺环境配置',load:async()=>({input,cores:[]}),save,back,proxyCheck:{scope,port:{targets,check}}}
  await act(async()=>{tree=create(<MemoryRouter><BrowserEditPage managed={managed}/></MemoryRouter>)});await choose()
  await act(async()=>button('检查此代理').props.onClick());expect(check).toHaveBeenCalledTimes(1)
  expect(save).not.toHaveBeenCalled();expect(back).not.toHaveBeenCalled()
  expect(native).not.toHaveBeenCalled()
  expect(tree.root.findAllByType('input').find(n=>n.props['aria-label']==='本地代理地址').props.value).toBe(proxyConfig)
  await act(async()=>tree.update(<MemoryRouter><BrowserEditPage managed={{...managed,proxyCheck:undefined}}/></MemoryRouter>))
  expect(button('检查此代理')).toBeUndefined()
})
