import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {act,create} from 'react-test-renderer'
import {Suspense} from 'react'
import {MemoryRouter,useLocation} from 'react-router-dom'
import {AuthGate} from '../src/auth/AuthGate'
import {acceptAuth,authSnapshot,beginAuthAction} from '../src/auth/session'
import {Layout} from '../src/shared/layout/Layout'
import {ShellSlotsProvider} from '../src/shared/layout/ShellSlots'
import {AppRoutes} from '../src/routes/AppRoutes'
import {navigationConfig} from '../src/config'
import {useLayoutStore} from '../src/store/layoutStore'
import {BusinessShell} from '../src/App'

const portals=vi.hoisted(()=>[])
vi.mock('react-dom',async original=>({...await original(),createPortal:(node,target)=>{portals.push(target);return node}}))
const text=node=>typeof node==='string'?node:node?.children?.map(text).join('')||''
const context=(id='org-a',revision=7)=>({revision,context:{user:{id:'synthetic'},effectiveOrganizationId:id,selectionRequired:false,organizations:['org-a','org-b'].map(id=>({id,name:id,permissions:['workbench.store.browser.use']}))}})
const input={profileName:'Synthetic Store',userDataDir:'',coreId:'',restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'',memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
const writes=['StoreEnvironmentSave','StoreEnvironmentOpen','StoreEnvironmentClose','BrowserProfileCreate','BrowserInstanceStart','BrowserKernelPrepare','StoreProxyCheck']
let tree,where,native,nativeSpies,status,generation,nodes,listeners,unexpected
function Location(){where=useLocation();return null}
const button=label=>tree.root.findAllByType('button').find(node=>text(node)===label||node.props['aria-label']===label)
const field=label=>tree.root.findAll(node=>typeof node.type==='string'&&node.props['aria-label']===label)[0]
const json=value=>Promise.resolve(JSON.stringify(value))
function focusable(){const node={isConnected:true,closest:()=>null,getClientRects:()=>[{}],focus:vi.fn(()=>{document.activeElement=node})};return node}
async function mount(path='/stores',completeShell=false){
 await act(async()=>{tree=create(<MemoryRouter initialEntries={[path]}><Location/><ShellSlotsProvider><AuthGate>{completeShell?<BusinessShell/>:<Layout><Suspense fallback={<p>Loading</p>}><AppRoutes/></Suspense></Layout>}</AuthGate></ShellSlotsProvider></MemoryRouter>,{createNodeMock:element=>{
  if(element.type==='aside')return nodes.aside
  if(element.props.id==='workspace-content')return nodes.main
  if(element.props.className==='shell-workspace')return nodes.workspace
  if(element.props.className==='shell-organization-slot')return nodes.organization
  if(element.type==='div'&&Object.keys(element.props).length===0)return nodes.account
  return null
 }})})
 await act(async()=>{await vi.dynamicImportSettled()})
}
beforeEach(()=>{
 vi.useFakeTimers();portals.length=0;unexpected=[];listeners={}
 generation=authSnapshot().generation+1;status={generation,state:'signed_in',configured:true,subject:'synthetic-user',expiresAt:new Date(Date.now()+60000).toISOString()}
 const first=focusable(),last=focusable(),trigger=focusable(),closedInput=focusable()
 closedInput.closest=()=>({querySelector:()=>last})
 nodes={first,last,trigger,main:focusable(),organization:{kind:'organization'},account:{kind:'account'},workspace:{inert:false},aside:{querySelector:()=>first,querySelectorAll:()=>[first,last,closedInput],addEventListener:(name,fn)=>{listeners[name]=fn},removeEventListener:(name)=>{delete listeners[name]}}}
 native={AuthStatus:vi.fn(()=>Promise.resolve(status)),GetAppConfig:vi.fn().mockResolvedValue({version:'fixture'}),
  BrowserProfileList:vi.fn().mockResolvedValue([]),ListGroups:vi.fn().mockResolvedValue([]),FrontendOperationLog:vi.fn().mockResolvedValue(undefined),
  StoreWorkbenchContext:vi.fn(()=>json(context())),StoreWorkbenchList:vi.fn(()=>json({items:[],pagination:{page:1,pageSize:20,total:0}})),
  StoreWorkbenchSelectOrganization:vi.fn(id=>json(context(id,8))),StoreEnvironmentOptions:vi.fn(()=>json({input,cores:[],binding:null})),BrowserKernelCatalog:vi.fn(()=>json([])),
  AuthGrantAPI:vi.fn(async(generation,granted)=>{status={...status,apiGranted:granted}}),
  AuthLogout:vi.fn(async()=>{status={...status,state:'signed_out'};return status}),
 }
 for(const method of writes)native[method]=vi.fn(()=>Promise.reject(new Error('unexpected effect')))
 nativeSpies={...native}
 const storage={getItem:()=>null,setItem:vi.fn(),removeItem:vi.fn()}
 vi.stubGlobal('window',{go:{main:{App:new Proxy(native,{get:(object,key)=>{if(!(key in object)){unexpected.push(String(key));return ()=>Promise.reject(new Error('unexpected method'))}return object[key]}})}},runtime:{EventsOnMultiple:()=>()=>{}},setInterval,clearInterval,setTimeout,clearTimeout,addEventListener:vi.fn(),removeEventListener:vi.fn(),localStorage:storage,sessionStorage:storage,location:{origin:'https://fixture.test',pathname:'/',search:'',hash:''}})
 vi.stubGlobal('document',{body:{style:{}},activeElement:trigger,querySelector:()=>null,getElementById:id=>id==='workspace-content'?nodes.main:null,addEventListener:vi.fn(),removeEventListener:vi.fn()})
 vi.stubGlobal('go',window.go)
 vi.stubGlobal('localStorage',storage)
 useLayoutStore.setState({sidebarCollapsed:false});acceptAuth(status,beginAuthAction())
})
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null;for(const method of writes)expect(nativeSpies[method],method).not.toHaveBeenCalled();expect(unexpected).toEqual([]);vi.useRealTimers();vi.unstubAllGlobals()})

it('places existing Store/auth owners in shell slots with one original read scope and retains every business entry',async()=>{
 await mount()
 expect(where.pathname).toBe('/stores');expect(portals).toContain(nodes.organization);expect(portals).toContain(nodes.account)
 expect(native.StoreWorkbenchContext).toHaveBeenCalledExactlyOnceWith(generation)
 expect(native.StoreWorkbenchList).toHaveBeenCalledExactlyOnceWith('','',1,7,generation)
 expect(native.GetAppConfig).toHaveBeenCalledTimes(1)
 for(const label of ['内核与扩展','更多工具','设置与备份'])await act(async()=>button(label).props.onClick())
 const paths=new Set(tree.root.findAllByType('a').map(node=>node.props.href).filter(Boolean))
 for(const item of navigationConfig.flatMap(section=>section.items))expect(paths.has(item.path),item.path).toBe(true)
 expect(paths.has('/profile')).toBe(true)
 expect(native.StoreWorkbenchSelectOrganization).not.toHaveBeenCalled();expect(native.AuthGrantAPI).not.toHaveBeenCalled();expect(native.AuthLogout).not.toHaveBeenCalled()
})
it('searches the actual Store list while preserving platform and unrelated query state',async()=>{
 await mount('/stores?platform=temu&page=2&return=kept')
 await act(async()=>field('全局搜索店铺').props.onChange({target:{value:'kept search'}}))
 await act(async()=>tree.root.findByProps({role:'search'}).props.onSubmit({preventDefault:vi.fn()}))
 const params=new URLSearchParams(where.search)
 expect(where.pathname).toBe('/stores');expect(params.get('q')).toBe('kept search');expect(params.get('platform')).toBe('temu');expect(params.get('page')).toBe('1');expect(params.get('return')).toBe('kept')
 expect(native.StoreWorkbenchList).toHaveBeenLastCalledWith('kept search','temu',1,7,generation)
 expect(native.StoreWorkbenchSelectOrganization).not.toHaveBeenCalled()
})
it('keeps editor search disabled and preserves dirty enterprise confirmation and cancel through the portal',async()=>{
 await mount('/stores/shop-a/environment/edit')
 expect(field('全局搜索店铺').props.disabled).toBe(true)
 await act(async()=>tree.root.findByProps({role:'search'}).props.onSubmit({preventDefault:vi.fn()}))
 await act(async()=>field('配置名称').props.onChange({target:{value:'kept draft'}}))
 await act(async()=>field('当前企业').props.onChange({target:{value:'org-b'}}))
 expect(text(tree.root)).toContain('放弃未保存的更改并切换企业？')
 await act(async()=>button('取消').props.onClick())
 expect(where.pathname).toBe('/stores/shop-a/environment/edit');expect(field('配置名称').props.value).toBe('kept draft')
 expect(native.StoreWorkbenchSelectOrganization).not.toHaveBeenCalled();expect(native.StoreWorkbenchList).not.toHaveBeenCalled()
})
it('keeps explicit enterprise selection single-flight with the original revision and generation',async()=>{
 await mount()
 const change=field('当前企业').props.onChange
 await act(async()=>{change({target:{value:'org-b'}});change({target:{value:'org-b'}})})
 expect(native.StoreWorkbenchSelectOrganization).toHaveBeenCalledExactlyOnceWith('org-b',7,generation)
 expect(native.StoreWorkbenchList).toHaveBeenLastCalledWith('','',1,8,generation)
})
it('skips to editor content without navigation, discard confirmation or clearing a dirty draft',async()=>{
 await mount('/stores/shop-a/environment/edit?return=kept')
 await act(async()=>field('配置名称').props.onChange({target:{value:'kept dirty draft'}}))
 const skip=button('跳转到内容');expect(skip.props.type).toBe('button');expect(skip.props.href).toBeUndefined()
 await act(async()=>skip.props.onClick())
 expect(document.activeElement).toBe(nodes.main);expect(where.pathname+where.search).toBe('/stores/shop-a/environment/edit?return=kept');expect(where.hash).toBe('')
 expect(tree.root.findAllByProps({role:'dialog'})).toHaveLength(0);expect(field('配置名称').props.value).toBe('kept dirty draft')
 await act(async()=>field('当前企业').props.onChange({target:{value:'org-b'}}))
 expect(text(tree.root)).toContain('放弃未保存的更改并切换企业？')
 expect(native.StoreWorkbenchSelectOrganization).not.toHaveBeenCalled()
})
it('retains the original explicit API permission handler inside the account portal',async()=>{
 await mount()
 const checkbox=tree.root.findByProps({type:'checkbox'})
 await act(async()=>checkbox.props.onChange({target:{checked:true}}))
 expect(native.AuthGrantAPI).toHaveBeenCalledExactlyOnceWith(generation,true)
 expect(native.AuthLogout).not.toHaveBeenCalled();expect(native.StoreWorkbenchSelectOrganization).not.toHaveBeenCalled()
})
it('retains explicit logout admission and unmounts Store content from the account portal',async()=>{
 await mount();await act(async()=>button('退出登录').props.onClick())
 expect(native.AuthLogout).toHaveBeenCalledExactlyOnceWith(generation)
 expect(button('通过浏览器登录')).toBeDefined();expect(tree.root.findAllByType('aside')).toHaveLength(0)
})
it('traps narrow navigation focus, closes on Escape, returns focus and clears background inert',async()=>{
 await mount();document.activeElement=nodes.trigger
 await act(async()=>button('打开导航').props.onClick())
 expect(tree.root.findByType('aside').props.role).toBe('dialog');expect(nodes.workspace.inert).toBe(true)
 document.activeElement=nodes.last;listeners.keydown({key:'Tab',preventDefault:vi.fn()});expect(document.activeElement).toBe(nodes.first)
 document.activeElement=nodes.first;listeners.keydown({key:'Tab',shiftKey:true,preventDefault:vi.fn()});expect(document.activeElement).toBe(nodes.last)
 await act(async()=>listeners.keydown({key:'Escape',preventDefault:vi.fn()}))
 expect(nodes.workspace.inert).toBe(false);expect(document.activeElement).toBe(nodes.trigger)
})
it('lets Store Ctrl K focus search without also opening the existing quick launcher, and retains Ctrl Shift K',async()=>{
 await mount('/stores',true)
 const handlers=window.addEventListener.mock.calls.filter(([name])=>name==='keydown').sort((a,b)=>Number(!!b[2])-Number(!!a[2])).map(([,fn])=>fn)
 const event=shiftKey=>({key:'k',ctrlKey:true,shiftKey,isComposing:false,defaultPrevented:false,preventDefault(){this.defaultPrevented=true}})
 await act(async()=>{const key=event(false);for(const handler of handlers)handler(key)})
 expect(tree.root.findAllByProps({role:'dialog'})).toHaveLength(0);expect(nativeSpies.BrowserProfileList).not.toHaveBeenCalled()
 await act(async()=>{const key=event(true);for(const handler of handlers)handler(key)})
 await act(async()=>{await vi.dynamicImportSettled()})
 expect(tree.root.findAllByProps({role:'dialog'})).toHaveLength(1)
 expect(nativeSpies.BrowserProfileList).toHaveBeenCalledExactlyOnceWith(generation)
 expect(nativeSpies.ListGroups).toHaveBeenCalledExactlyOnceWith(generation)
})
