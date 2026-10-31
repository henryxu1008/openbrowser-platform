import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {act,create} from 'react-test-renderer'
import {MemoryRouter,Route,Routes,useLocation,useNavigate} from 'react-router-dom'
import {StoreWorkspace,StoreListPage,StoreDetailPage,StoreEnvironmentEditPage} from '../src/modules/stores/StorePages'
import {acceptAuth,authSnapshot,beginAuthAction} from '../src/auth/session'
// Flatten only the renderer portal boundary. All production components,
// controls, adapters, auth capture and generated native bridge stay real.
vi.mock('react-dom',async original=>({...await original(),createPortal:children=>children}))
const release={releaseId:'10000000-0000-0000-0000-000000000001',version:'130.0.0.1',os:'windows',arch:'amd64',minClientVersion:'1.0.0',maxClientVersionExclusive:'',recommended:true,installed:false}
const input={profileName:'Shop A',userDataDir:'',coreId:'',restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'socks5://localhost:1080',memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
const context={revision:7,context:{user:{id:'synthetic'},effectiveOrganizationId:'org-a',selectionRequired:false,organizations:[{id:'org-a',name:'Org A',permissions:['workbench.store.browser.use']}]}}
const state={binding:{bindingId:'binding-a',profileId:'profile-a',bindingVersion:'4'},name:'Shop A',coreId:release.releaseId,proxySummary:'socks5',running:false,runtimeKnown:true,pending:null,lastOperation:null,reports:[],observedAt:''}
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no});return{promise,resolve,reject}}
let tree,location,navigate,native,off,generation
const text=node=>typeof node==='string'?node:node?.children?.map(text).join('')||''
const button=label=>tree.root.findAllByType('button').find(node=>text(node)===label)
const field=label=>tree.root.findAll(node=>typeof node.type==='string'&&node.props['aria-label']===label)[0]
function Location(){location=useLocation();navigate=useNavigate();return null}
async function mount(path='/stores/shop-a/environment/edit?return=q%3Dkept'){await act(async()=>{tree=create(<MemoryRouter initialEntries={[path]}><Location/><Routes><Route path="/stores" element={<StoreWorkspace/>}><Route index element={<StoreListPage/>}/><Route path=":storeId" element={<StoreDetailPage/>}/><Route path=":storeId/environment/edit" element={<StoreEnvironmentEditPage/>}/></Route><Route path="/elsewhere" element={<p>elsewhere</p>}/></Routes></MemoryRouter>)})}
beforeEach(()=>{
 vi.useFakeTimers();const json=value=>Promise.resolve(JSON.stringify(value))
 native={StoreWorkbenchContext:vi.fn(()=>json(context)),StoreEnvironmentOptions:vi.fn(()=>json({input,cores:[],binding:state.binding})),BrowserKernelCatalog:vi.fn(()=>json([release])),BrowserKernelPrepare:vi.fn().mockResolvedValue(undefined),StoreEnvironmentSave:vi.fn(()=>json(state)),StoreWorkbenchGet:vi.fn(id=>json({id,name:'Shop',platform:'amazon',region:'US',recordStatus:'active'})),StoreEnvironmentGet:vi.fn(()=>json(state)),StoreAuditList:vi.fn(()=>json({items:[],nextCursor:null,coverage:[],canFilterActor:false})),StoreEnvironmentOpen:vi.fn(),StoreEnvironmentClose:vi.fn()}
 off=vi.fn()
 vi.stubGlobal('window',{go:{main:{App:new Proxy(native,{get:(target,key)=>{if(!(key in target))throw new Error(`Unexpected native method: ${String(key)}`);return target[key]}})}},runtime:{EventsOnMultiple:vi.fn(()=>off)},location:{origin:'https://app.test',pathname:'/stores/shop-a/environment/edit',search:'',hash:''},setTimeout,clearTimeout,addEventListener:vi.fn(),removeEventListener:vi.fn()})
 vi.stubGlobal('document',{body:{style:{}},activeElement:null,addEventListener:vi.fn(),removeEventListener:vi.fn()})
 generation=authSnapshot().generation+1;acceptAuth({generation,state:'signed_in',configured:true,expiresAt:new Date(Date.now()+60000).toISOString()},beginAuthAction())
})
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null;vi.useRealTimers();vi.unstubAllGlobals()})
const choose=()=>field('内核版本').props.onChange({target:{value:release.releaseId}})
it.each([['existing',state.binding,'4'],['new',null,'0']])('prepares once and saves %s environment with original Store CAS without opening',async(kind,binding,expectedVersion)=>{
 native.StoreEnvironmentOptions.mockResolvedValue(JSON.stringify({input,cores:[],binding}))
 const prepare=deferred(),save=deferred();native.BrowserKernelPrepare.mockReturnValue(prepare.promise);native.StoreEnvironmentSave.mockReturnValue(save.promise)
 await mount();expect(native.StoreEnvironmentOptions).toHaveBeenCalledWith('shop-a',7,generation);expect(native.BrowserKernelPrepare).not.toHaveBeenCalled()
 await act(async()=>{choose();choose();button('保存配置').props.onClick()});expect(native.BrowserKernelPrepare).toHaveBeenCalledExactlyOnceWith(release.releaseId,generation);expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(button('保存配置').props.disabled).toBe(true)
 await act(async()=>prepare.resolve());expect(field('内核版本').props.value).toBe(release.releaseId)
 await act(async()=>{button('保存配置').props.onClick();button('保存配置').props.onClick()});expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
 const [store,version,raw,revision,auth]=native.StoreEnvironmentSave.mock.calls[0];expect([store,version,revision,auth]).toEqual(['shop-a',expectedVersion,7,generation]);expect(JSON.parse(raw)).toEqual({...input,coreId:release.releaseId})
 await act(async()=>save.resolve(JSON.stringify(state)));expect(location.pathname+location.search).toBe('/stores/shop-a?q=kept');expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled();expect(native.StoreEnvironmentClose).not.toHaveBeenCalled()
})
it('keeps prior selection on preparation failure and retries only explicitly',async()=>{
 native.StoreEnvironmentOptions.mockResolvedValue(JSON.stringify({input:{...input,coreId:'prior-release'},cores:[],binding:state.binding}));native.BrowserKernelPrepare.mockRejectedValue(new Error('kernel_artifact_integrity_failed'))
 await mount();await act(async()=>choose());expect(field('内核版本').props.value).toBe('prior-release');expect(text(tree.root)).toContain('内核制品校验失败');expect(native.StoreEnvironmentSave).not.toHaveBeenCalled()
 native.BrowserKernelPrepare.mockResolvedValue(undefined);await act(async()=>choose());expect(native.BrowserKernelPrepare).toHaveBeenCalledTimes(2);expect(field('内核版本').props.value).toBe(release.releaseId);expect(native.StoreEnvironmentSave).not.toHaveBeenCalled()
})
it.each(['return','other-store','unmount'])('ignores late preparation after %s without saving or changing destination',async kind=>{
 const pending=deferred();native.BrowserKernelPrepare.mockReturnValue(pending.promise);await mount();await act(async()=>{choose()});expect(native.BrowserKernelPrepare).toHaveBeenCalledTimes(1)
 await act(async()=>{if(kind==='return')button('返回店铺').props.onClick();else if(kind==='other-store')navigate('/stores/shop-b/environment/edit');else{tree.unmount();tree=null}})
 await act(async()=>pending.resolve());expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled();if(kind==='return')expect(location.pathname).toBe('/stores/shop-a');if(kind==='other-store'){expect(location.pathname).toBe('/stores/shop-b/environment/edit');expect(field('内核版本').props.value).toBe('')}expect(off).toHaveBeenCalledTimes(1)
})
it('keeps editing on cancel and discards late preparation only after confirmed return',async()=>{
 const pending=deferred();native.BrowserKernelPrepare.mockReturnValue(pending.promise)
 await mount();act(()=>field('配置名称').props.onChange({target:{value:'Unsaved'}}));await act(async()=>choose())
 act(()=>button('返回店铺').props.onClick());expect(text(tree.root)).toContain('放弃未保存的更改')
 act(()=>button('继续编辑').props.onClick());expect(location.pathname).toBe('/stores/shop-a/environment/edit');expect(field('配置名称').props.value).toBe('Unsaved')
 act(()=>button('返回店铺').props.onClick());act(()=>button('放弃并离开').props.onClick());await act(async()=>vi.advanceTimersByTime(420))
 expect(location.pathname).toBe('/stores/shop-a');await act(async()=>pending.resolve());expect(location.pathname).toBe('/stores/shop-a');expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it('does not navigate back to the old Store when an in-flight save resolves after leaving',async()=>{
 const pending=deferred();native.StoreEnvironmentSave.mockReturnValue(pending.promise)
 await mount();await act(async()=>choose());await act(async()=>{button('保存配置').props.onClick()});expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
 await act(async()=>navigate('/stores/shop-b'));await act(async()=>pending.resolve(JSON.stringify(state)))
 expect(location.pathname).toBe('/stores/shop-b');expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1);expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it.each([['BINDING_VERSION_CONFLICT',false],['store_result_unknown operationId=10000000-0000-0000-0000-000000000099',true]])('preserves inputs after native save error %s without retry or navigation',async(error,blocked)=>{
 native.StoreEnvironmentSave.mockRejectedValue(new Error(error))
 await mount();await act(async()=>choose());await act(async()=>{button('保存配置').props.onClick()})
 expect(location.pathname).toBe('/stores/shop-a/environment/edit');expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
 expect(field('本地代理地址').props.value).toBe(input.proxyConfig);expect(field('内核版本').props.value).toBe(release.releaseId)
 expect(button('保存配置').props.disabled).toBe(blocked);expect(text(tree.root)).toContain(blocked?'结果待核对':'返回店铺核对')
 expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it('reuses only the current Store network and requires an explicit direct selection',async()=>{
 await mount();expect(field('网络使用方式').props.value).toBe('current');expect(field('本地代理地址').props.value).toBe(input.proxyConfig)
 act(()=>field('网络使用方式').props.onChange({target:{value:'direct'}}));expect(field('本地代理地址').props.value).toBe('')
 act(()=>field('网络使用方式').props.onChange({target:{value:'current'}}));expect(field('本地代理地址').props.value).toBe(input.proxyConfig)
 await act(async()=>choose());await act(async()=>button('保存配置').props.onClick())
 expect(JSON.parse(native.StoreEnvironmentSave.mock.calls[0][2]).proxyConfig).toBe(input.proxyConfig)
 expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled();expect(text(tree.root)).not.toContain('网络正常')
})
it('ignores captured network selections during save and after leaving the editor',async()=>{
 const pending=deferred();native.StoreEnvironmentSave.mockReturnValue(pending.promise);await mount();await act(async()=>choose())
 const select=field('网络使用方式').props.onChange
 await act(async()=>{button('保存配置').props.onClick();select({target:{value:'direct'}})})
 expect(field('本地代理地址').props.value).toBe(input.proxyConfig);expect(field('网络使用方式').props.disabled).toBe(true)
 await act(async()=>navigate('/stores/shop-b'));act(()=>select({target:{value:'direct'}}));await act(async()=>pending.resolve(JSON.stringify(state)))
 expect(location.pathname).toBe('/stores/shop-b');expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
})

it('renders only authorized catalog facts and retries a failed list read explicitly',async()=>{
 native.StoreWorkbenchList=vi.fn().mockRejectedValueOnce(new Error('unavailable')).mockResolvedValue(JSON.stringify({items:[{id:'shop-a',name:'Shop A',platform:'temu',region:'US',recordStatus:'active'}],pagination:{page:1,pageSize:20,total:1}}))
 await mount('/stores?q=kept&platform=temu');expect(native.StoreWorkbenchList).toHaveBeenCalledExactlyOnceWith('kept','temu',1,7,generation)
 expect(tree.root.findAllByType('td')).toHaveLength(0);await act(async()=>button('重新读取店铺').props.onClick())
 expect(native.StoreWorkbenchList).toHaveBeenCalledTimes(2);expect(text(tree.root)).toContain('1 家可访问店铺');expect(text(tree.root)).toContain('已启用');expect(text(tree.root)).not.toContain('网络正常')
 expect(native.StoreEnvironmentGet).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
 const link=tree.root.findAllByType('a').find(n=>text(n)==='进入工作台');expect(link.props.href).toContain('return=q%3Dkept%26platform%3Dtemu')
})
it('keeps query controls aligned with history and ignores an older catalog response',async()=>{
 const old=deferred();native.StoreWorkbenchList=vi.fn().mockReturnValueOnce(old.promise).mockResolvedValue(JSON.stringify({items:[],pagination:{page:1,pageSize:20,total:0}}))
 await mount('/stores?q=old&platform=temu');await act(async()=>navigate('/stores?q=new&platform=shein'))
 expect(field('搜索店铺').props.value).toBe('new');expect(field('平台').props.value).toBe('shein')
 await act(async()=>old.resolve(JSON.stringify({items:[{id:'old',name:'STALE STORE'}],pagination:{page:1,pageSize:20,total:999}})))
 expect(text(tree.root)).not.toContain('STALE STORE');expect(text(tree.root)).toContain('0 家可访问店铺')
 await act(async()=>navigate(-1));expect(field('搜索店铺').props.value).toBe('old');expect(field('平台').props.value).toBe('temu')
})
it('saves an explicit direct network without changing binding or automatically opening',async()=>{
 await mount();act(()=>field('网络使用方式').props.onChange({target:{value:'direct'}}));await act(async()=>choose());await act(async()=>button('保存配置').props.onClick())
 const [id,version,payload,revision,epoch]=native.StoreEnvironmentSave.mock.calls[0];expect([id,version,revision,epoch]).toEqual(['shop-a','4',7,generation]);expect(JSON.parse(payload)).toMatchObject({proxyId:'',proxyConfig:''});expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it('does not silently turn an empty custom network into a direct connection',async()=>{
 await mount();await act(async()=>choose());act(()=>field('本地代理地址').props.onChange({target:{value:'   '}}));await act(async()=>button('保存配置').props.onClick())
 expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(text(tree.root)).toContain('明确使用直连');expect(location.pathname).toContain('/environment/edit')
})
