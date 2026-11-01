import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {act,create} from 'react-test-renderer'
import {Suspense} from 'react'
import {MemoryRouter,useLocation} from 'react-router-dom'
import {AuthGate} from '../src/auth/AuthGate'
import {acceptAuth,authSnapshot,beginAuthAction} from '../src/auth/session'
import {AppRoutes} from '../src/routes/AppRoutes'

// Real gate, routes, Store pages, lazy standalone page, adapters and generated
// bridge. Only native calls/events and the renderer portal boundary are mocked.
vi.mock('react-dom',async original=>({...await original(),createPortal:children=>children}))
const writes=['AuthLogin','AuthLogout','AuthCancel','AuthGrantAPI','QuitAppOnly',
 'StoreWorkbenchSelectOrganization','StoreEnvironmentSave','StoreEnvironmentOpen','StoreEnvironmentClose',
 'BrowserProfileCreate','BrowserInstanceStart','StartBrowserInstance','BrowserKernelPrepare']
const input={profileName:'Synthetic Store',userDataDir:'',coreId:'',restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'',memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
const store={id:'shop-a',name:'Synthetic Store',platform:'temu',region:'US',recordStatus:'active'}
const environment={binding:null,name:'',coreId:'',proxySummary:'',running:false,runtimeKnown:true,pending:null,lastOperation:null,reports:[],observedAt:''}
const context=(selected=true)=>({revision:7,context:{user:{id:'synthetic'},effectiveOrganizationId:selected?'org-a':null,selectionRequired:!selected,organizations:[{id:'org-a',name:'Synthetic Org',permissions:['workbench.store.browser.use']}]}})
let tree,where,native,currentStatus,generation,unexpected
const text=node=>typeof node==='string'?node:node?.children?.map(text).join('')||''
function Location(){where=useLocation();return null}
const button=label=>tree.root.findAllByType('button').find(node=>text(node)===label)
const json=value=>Promise.resolve(JSON.stringify(value))
async function mount(path='/'){
 await act(async()=>{tree=create(<MemoryRouter initialEntries={[path]}><Location/><AuthGate><Suspense fallback={<p>Loading route</p>}><AppRoutes/></Suspense></AuthGate></MemoryRouter>)})
 await act(async()=>{await vi.dynamicImportSettled()})
}
beforeEach(()=>{
 generation=authSnapshot().generation+1
 currentStatus={generation,state:'signed_in',configured:true,subject:'synthetic',expiresAt:new Date(Date.now()+60000).toISOString()}
 unexpected=[]
 native={AuthStatus:vi.fn(()=>Promise.resolve(currentStatus)),
  StoreWorkbenchContext:vi.fn(()=>json(context())),
  StoreWorkbenchList:vi.fn(()=>json({items:[store],pagination:{page:1,pageSize:20,total:1}})),
  StoreWorkbenchGet:vi.fn(()=>json(store)),StoreEnvironmentGet:vi.fn(()=>json(environment)),
  StoreEnvironmentOptions:vi.fn(()=>json({input,cores:[],binding:null})),
  StoreAuditList:vi.fn(()=>json({items:[],nextCursor:null,coverage:[],canFilterActor:false})),
  BrowserKernelCatalog:vi.fn(()=>json([])),BrowserProfileList:vi.fn().mockResolvedValue([]),
  ListGroups:vi.fn().mockResolvedValue([]),BrowserProxyList:vi.fn().mockResolvedValue([]),BrowserCoreList:vi.fn().mockResolvedValue([]),
 }
 for(const method of writes)native[method]=vi.fn(()=>Promise.reject(new Error(`Unexpected write: ${method}`)))
 const storage={getItem:vi.fn(()=>null),setItem:vi.fn(),removeItem:vi.fn()}
 const app=new Proxy(native,{get:(target,key)=>{
  if(!(key in target)){unexpected.push(String(key));return ()=>Promise.reject(new Error(`Unexpected native method: ${String(key)}`))}
  return target[key]
 }})
 vi.stubGlobal('window',{go:{main:{App:app}},runtime:{EventsOnMultiple:()=>()=>{}},
  location:{origin:'https://fixture.test',pathname:'/',search:'',hash:''},localStorage:storage,sessionStorage:storage,
  setInterval,clearInterval,setTimeout,clearTimeout,addEventListener:vi.fn(),removeEventListener:vi.fn()})
 vi.stubGlobal('document',{body:{style:{}},activeElement:null,visibilityState:'visible',addEventListener:vi.fn(),removeEventListener:vi.fn()})
 vi.stubGlobal('localStorage',storage)
 acceptAuth(currentStatus,beginAuthAction())
})
afterEach(()=>{
 if(tree)act(()=>tree.unmount());tree=null
 for(const method of writes)expect(native[method],method).not.toHaveBeenCalled()
 expect(unexpected).toEqual([])
 vi.unstubAllGlobals()
})

it.each([false,true])('enters the Store workspace from an authenticated root, selected enterprise=%s',async selected=>{
 native.StoreWorkbenchContext.mockResolvedValue(JSON.stringify(context(selected)))
 await mount()
 expect(where.pathname).toBe('/stores');expect(where.search).toBe('')
 expect(native.StoreWorkbenchContext).toHaveBeenCalledExactlyOnceWith(generation)
 expect(tree.root.findAllByType('select').some(node=>node.props['aria-label']==='当前企业')).toBe(true)
 if(selected){
  expect(text(tree.root)).toContain('Synthetic Store')
  expect(native.StoreWorkbenchList).toHaveBeenCalledExactlyOnceWith('','',1,7,generation)
 }else{
  expect(text(tree.root)).toContain('请选择企业后查看获授权的店铺')
  expect(native.StoreWorkbenchList).not.toHaveBeenCalled()
 }
 expect(native.BrowserProfileList).not.toHaveBeenCalled()
})
it('keeps root behind the login gate and enters Stores only after a verified sign-in snapshot',async()=>{
 currentStatus={...currentStatus,state:'signed_out'};acceptAuth(currentStatus,beginAuthAction())
 await mount()
 expect(where.pathname).toBe('/');expect(button('通过浏览器登录')).toBeDefined()
 expect(native.StoreWorkbenchContext).not.toHaveBeenCalled();expect(native.BrowserProfileList).not.toHaveBeenCalled()
 currentStatus={...currentStatus,state:'signed_in',generation:generation+1}
 await act(async()=>{acceptAuth(currentStatus,beginAuthAction())})
 expect(where.pathname).toBe('/stores')
 expect(native.StoreWorkbenchContext).toHaveBeenCalledExactlyOnceWith(generation+1)
 expect(native.BrowserProfileList).not.toHaveBeenCalled()
})
it('keeps a denied Store context blocked at the default destination without a standalone fallback',async()=>{
 native.StoreWorkbenchContext.mockRejectedValue(new Error('PERMISSION_DENIED'))
 await mount()
 expect(where.pathname).toBe('/stores');expect(text(tree.root)).toContain('当前身份无权访问该店铺或执行此操作')
 expect(native.StoreWorkbenchList).not.toHaveBeenCalled();expect(native.BrowserProfileList).not.toHaveBeenCalled()
 expect(text(tree.root)).not.toContain('新建配置')
})
it('preserves the explicitly selected standalone list and its query',async()=>{
 await mount('/browser/list?view=table')
 expect(where.pathname+where.search).toBe('/browser/list?view=table')
 expect(native.BrowserProfileList).toHaveBeenCalledExactlyOnceWith(generation)
 expect(native.StoreWorkbenchContext).not.toHaveBeenCalled();expect(text(tree.root)).toContain('实例列表')
})
it.each([
 ['/stores/shop-a?tab=records&return=q%3Dkept','StoreAuditList'],
 ['/stores/shop-a/environment/edit?return=q%3Dkept','StoreEnvironmentOptions'],
])('preserves the explicit Store deep link %s',async(path,read)=>{
 await mount(path)
 expect(where.pathname+where.search).toBe(path)
 expect(native.StoreWorkbenchContext).toHaveBeenCalledExactlyOnceWith(generation)
 expect(native[read]).toHaveBeenCalled()
 expect(native.BrowserProfileList).not.toHaveBeenCalled()
})
