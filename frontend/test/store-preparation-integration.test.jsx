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
const proxyOptions={available:true,reason:'NONE',catalogVersion:'c'.repeat(64),targets:[{id:'fixture',label:'离线示例目标',url:'https://target.example.invalid/check'}]}
const proxyResult=request=>({schema:'store-proxy-check-v1',requestId:request.requestId,targetId:request.targetId,syntax:'passed',tcp:'passed',target:'passed',reason:'NONE'})
function withProxyOptions(options=proxyOptions){
 native.StoreEnvironmentOptions.mockResolvedValue(JSON.stringify({input,cores:[],binding:state.binding,proxyCheck:options}))
 native.StoreProxyCheck=vi.fn((id,version,raw)=>Promise.resolve(JSON.stringify(proxyResult(JSON.parse(raw)))))
 native.StoreProxyCheckCancel=vi.fn((id,requestId)=>Promise.resolve(JSON.stringify({requestId,settled:false})))
}
it('shows publisher target configuration as missing, with zero native check or save calls',async()=>{
 withProxyOptions({available:false,reason:'TARGET_CONFIGURATION_REQUIRED',catalogVersion:'',targets:[]});await mount()
 expect(text(tree.root)).toContain('此版本尚未由发布方配置检查目标')
 expect(button('检查此代理').props.disabled).toBe(true)
 expect(native.StoreProxyCheck).not.toHaveBeenCalled();expect(native.StoreEnvironmentSave).not.toHaveBeenCalled()
})
it('calls the dedicated real bridge once with original Store CAS, revision and auth generation',async()=>{
 withProxyOptions();await mount();await act(async()=>field('检查目标').props.onChange({target:{value:'fixture'}}))
 await act(async()=>{button('检查此代理').props.onClick();button('检查此代理').props.onClick()})
 expect(native.StoreProxyCheck).toHaveBeenCalledTimes(1)
 const [id,version,raw,revision,g]=native.StoreProxyCheck.mock.calls[0],request=JSON.parse(raw)
 expect([id,version,revision,g]).toEqual(['shop-a','4',7,generation])
 expect(request).toEqual({requestId:expect.any(String),proxyConfig:input.proxyConfig,targetId:'fixture',catalogVersion:proxyOptions.catalogVersion,expiresAtUnixMs:expect.any(Number)})
 expect(text(tree.root)).toContain('代理服务器 TCP')
 for(const method of ['StoreEnvironmentSave','StoreEnvironmentOpen','StoreEnvironmentClose','BrowserKernelPrepare'])expect(native[method]).not.toHaveBeenCalled()
 expect(field('本地代理地址').props.value).toBe(input.proxyConfig)
})
it('does not offer a proxy check for an explicitly direct environment',async()=>{
 withProxyOptions();await mount();await act(async()=>field('网络使用方式').props.onChange({target:{value:'direct'}}))
 expect(button('检查此代理')).toBeUndefined();expect(native.StoreProxyCheck).not.toHaveBeenCalled();expect(native.StoreEnvironmentSave).not.toHaveBeenCalled()
})
it('keeps credentials and target unknown when a configured connector bridge is required',async()=>{
 withProxyOptions();native.StoreEnvironmentOptions.mockResolvedValue(JSON.stringify({input:{...input,proxyConfig:'http://fixture-user:fixture-secret@proxy.example.invalid:8080'},cores:[],binding:state.binding,proxyCheck:proxyOptions}))
 native.StoreProxyCheck.mockImplementation((id,v,raw)=>Promise.resolve(JSON.stringify({...proxyResult(JSON.parse(raw)),target:'not_checked',reason:'CONNECTOR_REQUIRED'})))
 await mount();await act(async()=>field('检查目标').props.onChange({target:{value:'fixture'}}));await act(async()=>button('检查此代理').props.onClick())
 expect(text(tree.root)).toContain('尚未检查凭据和所选目标');expect(text(tree.root)).not.toContain('fixture-secret')
 expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it.each(['cancel','input','unmount'])('cancels original operation over real bridge on %s and ignores late success',async mode=>{
 withProxyOptions();const pending=deferred();native.StoreProxyCheck.mockReturnValue(pending.promise)
 await mount();await act(async()=>field('检查目标').props.onChange({target:{value:'fixture'}}));await act(async()=>button('检查此代理').props.onClick())
 const request=JSON.parse(native.StoreProxyCheck.mock.calls[0][2])
 await act(async()=>{if(mode==='cancel')button('取消检查').props.onClick();else if(mode==='input')field('本地代理地址').props.onChange({target:{value:'http://other.example.invalid:8080'}});else{tree.unmount();tree=null}})
 expect(native.StoreProxyCheckCancel).toHaveBeenCalledExactlyOnceWith('shop-a',request.requestId,7,generation)
 if(tree)expect(button('检查此代理').props.disabled).toBe(true)
 await act(async()=>pending.resolve(JSON.stringify(proxyResult(request))))
 if(tree)expect(text(tree.root)).not.toContain('代理服务器 TCP')
 expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
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
it.each(['catalog','preparation'])('explains kernel permission denial during %s and preserves the draft until an explicit refresh',async stage=>{
 const pending=deferred(),prior='prior-release'
 native.StoreEnvironmentOptions.mockResolvedValue(JSON.stringify({input:{...input,coreId:prior},cores:[],binding:state.binding}))
 if(stage==='catalog')native.BrowserKernelCatalog.mockReturnValue(pending.promise)
 else native.BrowserKernelPrepare.mockReturnValueOnce(pending.promise)
 await mount()
 if(stage==='preparation')await act(async()=>{choose();choose()})
 const catalogCalls=native.BrowserKernelCatalog.mock.calls.length
 expect(field('内核版本').props.disabled).toBe(true)
 expect(button('刷新版本').props.disabled).toBe(true)
 if(stage==='preparation')expect(button('保存配置').props.disabled).toBe(true)
 const denial='kernel_release_forbidden synthetic-private-token https://private.example.invalid/kernel'
 await act(async()=>pending.reject(stage==='catalog'?denial:new Error(denial)))
 const message='当前账号无法读取此内核发行信息，请联系平台维护者核对访问权限；处理后刷新版本列表。'
 expect(tree.root.findAllByProps({role:'alert'}).map(text)).toContain(message)
 expect(text(tree.root)).not.toContain('synthetic-private-token')
 expect(text(tree.root)).not.toContain('private.example.invalid')
 expect(text(tree.root)).not.toContain('所选版本已就绪')
 expect(field('内核版本').props.value).toBe(prior)
 expect(field('本地代理地址').props.value).toBe(input.proxyConfig)
 expect(field('内核版本').props.disabled).toBe(false)
 expect(button('刷新版本').props.disabled).toBe(false)
 expect(button('保存配置').props.disabled).toBe(false)
 expect(location.pathname).toBe('/stores/shop-a/environment/edit')
 await act(async()=>vi.advanceTimersByTime(1000))
 expect(native.BrowserKernelCatalog).toHaveBeenCalledTimes(catalogCalls)
 expect(native.BrowserKernelPrepare).toHaveBeenCalledTimes(stage==='catalog'?0:1)
 if(stage==='preparation')expect(off).toHaveBeenCalledTimes(1)
 native.BrowserKernelCatalog.mockResolvedValue(JSON.stringify([release]))
 await act(async()=>button('刷新版本').props.onClick())
 expect(native.BrowserKernelCatalog).toHaveBeenCalledTimes(catalogCalls+1)
 expect(native.BrowserKernelPrepare).toHaveBeenCalledTimes(stage==='catalog'?0:1)
 expect(field('内核版本').props.value).toBe(prior)
 expect(tree.root.findAllByProps({role:'alert'})).toHaveLength(0)
 for(const method of ['StoreEnvironmentSave','StoreEnvironmentOpen','StoreEnvironmentClose'])expect(native[method]).not.toHaveBeenCalled()
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
 expect(storePortal()).toBeDefined()
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
it.each(['resolve','reject'])('keeps the new Store network and CAS isolated when old options %s late',async outcome=>{
 const old=deferred(),bindingB={bindingId:'binding-b',profileId:'profile-b',bindingVersion:'19'}
 const inputB={...input,profileName:'Shop B',proxyConfig:'http://localhost:2080'}
 native.StoreEnvironmentOptions.mockImplementation(id=>id==='shop-a'?old.promise:Promise.resolve(JSON.stringify({input:inputB,cores:[],binding:bindingB})))
 await mount();expect(button('保存配置').props.disabled).toBe(true)
 expect(native.StoreEnvironmentOptions).toHaveBeenCalledExactlyOnceWith('shop-a',7,generation)
 await act(async()=>navigate('/stores/shop-b/environment/edit?return=q%3Db'))
 expect(field('配置名称').props.value).toBe('Shop B');expect(field('本地代理地址').props.value).toBe(inputB.proxyConfig)
 await act(async()=>{if(outcome==='resolve')old.resolve(JSON.stringify({input,cores:[],binding:state.binding}));else old.reject(new Error('old Store options failed'))})
 expect(field('配置名称').props.value).toBe('Shop B');expect(text(tree.root)).not.toContain('环境配置暂不可用')
 act(()=>field('网络使用方式').props.onChange({target:{value:'direct'}}))
 act(()=>field('网络使用方式').props.onChange({target:{value:'current'}}))
 expect(field('本地代理地址').props.value).toBe(inputB.proxyConfig)
 await act(async()=>choose());await act(async()=>button('保存配置').props.onClick())
 expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
 const [id,version,payload,revision,epoch]=native.StoreEnvironmentSave.mock.calls[0]
 expect([id,version,revision,epoch]).toEqual(['shop-b','19',7,generation])
 expect(JSON.parse(payload)).toEqual({...inputB,coreId:release.releaseId})
 expect(location.pathname+location.search).toBe('/stores/shop-b?q=b')
 expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled();expect(native.StoreEnvironmentClose).not.toHaveBeenCalled()
})

const storePortal = () => tree.root.findAllByType('div').find(node => node.props.className?.split(' ').includes('store-workbench-modal'))
it('keeps the managed leave confirmation in the Store portal scope and cancels without saving',async()=>{
 await mount();act(()=>field('配置名称').props.onChange({target:{value:'Unsaved'}}))
 act(()=>button('返回店铺').props.onClick());expect(storePortal()).toBeDefined()
 act(()=>button('继续编辑').props.onClick());await act(async()=>vi.advanceTimersByTime(680))
 expect(location.pathname).toBe('/stores/shop-a/environment/edit');expect(field('配置名称').props.value).toBe('Unsaved')
 expect(native.StoreEnvironmentSave).not.toHaveBeenCalled()
})
it('keeps the dirty enterprise confirmation scoped and selects once with the original revision',async()=>{
 const other={id:'org-b',name:'Org B',permissions:['workbench.store.browser.use']}
 native.StoreWorkbenchContext.mockResolvedValue(JSON.stringify({...context,context:{...context.context,organizations:[...context.context.organizations,other]}}))
 native.StoreWorkbenchSelectOrganization=vi.fn().mockResolvedValue(JSON.stringify({...context,revision:8,context:{...context.context,effectiveOrganizationId:'org-b'}}))
 await mount();act(()=>field('配置名称').props.onChange({target:{value:'Unsaved'}}))
 act(()=>field('当前企业').props.onChange({target:{value:'org-b'}}));expect(storePortal()).toBeDefined()
 const confirm=tree.root.findByProps({role:'dialog'}).findAllByType('button').at(-1).props.onClick
 act(()=>{confirm();confirm()})
 expect(tree.root.findByProps({role:'dialog'}).findAllByType('button').slice(-2).every(node=>node.props.disabled)).toBe(true)
 await act(async()=>vi.advanceTimersByTime(420))
 expect(native.StoreWorkbenchSelectOrganization).toHaveBeenCalledExactlyOnceWith('org-b',7,generation)
 expect(native.StoreEnvironmentSave).not.toHaveBeenCalled()
})
it.each(['record','receipt'])('keeps the read-only %s dialog in the Store portal scope',async kind=>{
 native.StoreAuditList.mockResolvedValue(JSON.stringify({items:[{eventId:'event-a',operationId:'op-a',action:'browser_open_reported',actorSubject:'fixture',serverTime:'2026-10-31T10:40:35Z',outcome:'failed',factSource:'client_report'}],nextCursor:null,coverage:[],canFilterActor:false}))
 native.StoreEnvironmentGet.mockResolvedValue(JSON.stringify({...state,lastOperation:{operationId:'op-a',outcome:'failed',reportState:'confirmed'}}))
 native.StoreOperationGet=vi.fn().mockResolvedValue(JSON.stringify({operationId:'op-a',outcome:'failed'}))
 await mount(`/stores/shop-a${kind==='record'?'?tab=records':''}`)
 await act(async()=>button(kind==='record'?'查看记录':'只读核对原操作').props.onClick())
 expect(storePortal()).toBeDefined();expect(tree.root.findByProps({role:'dialog'}).props['aria-labelledby']).toBeTruthy()
 if(kind==='receipt')expect(native.StoreOperationGet).toHaveBeenCalledExactlyOnceWith('shop-a','op-a',7,generation)
 expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled();expect(native.StoreEnvironmentClose).not.toHaveBeenCalled()
})

const parameterForm = () => tree.root.findByProps({'aria-label':'自有代理参数'})
const fillParameters = (values={}) => {
  act(()=>button('填写代理参数').props.onClick())
  for (const [label,value] of Object.entries({'代理协议':'http','代理地址':'proxy.example.invalid','代理端口':'8080','代理账号':'fixture-user','代理密码':'fixture-secret',...values})) {
    act(()=>field(label).props.onChange({target:{value}}))
  }
}
const submitParameters = () => parameterForm().props.onSubmit({preventDefault(){}})
const expectNoProxyEffects = () => {
  for(const method of ['StoreEnvironmentSave','StoreEnvironmentOpen','StoreEnvironmentClose','StoreProxyCheck','BrowserKernelPrepare']) if(native[method]) expect(native[method]).not.toHaveBeenCalled()
}
it('applies encoded parameters once to the masked Store draft, with no save/start/probe/pool effects',async()=>{
  withProxyOptions();await mount()
  act(()=>button('填写代理参数').props.onClick())
  for(const [label,value] of Object.entries({'代理协议':'socks5','代理地址':'::1','代理端口':'1080','代理账号':'合成 user:@%','代理密码':' p:@%/?# 合成 '}))act(()=>field(label).props.onChange({target:{value}}))
  expect(field('代理密码').props.type).toBe('password')
  const apply=parameterForm().props.onSubmit
  act(()=>{apply({preventDefault(){}});apply({preventDefault(){}})})
  const raw=field('本地代理地址'),url=new URL(raw.props.value)
  expect(raw.props.type).toBe('password');expect(url.hostname).toBe('[::1]')
  expect(decodeURIComponent(url.username)).toBe('合成 user:@%');expect(decodeURIComponent(url.password)).toBe(' p:@%/?# 合成 ')
  expect(field('网络使用方式').props.value).toBe('custom')
  expect(text(tree.root)).toContain('尚未保存或检测');expect(text(tree.root)).not.toContain('p:@%/?#')
  expect(tree.root.findAllByProps({'aria-label':'自有代理参数'})).toHaveLength(0)
  expectNoProxyEffects()
  act(()=>button('返回店铺').props.onClick());expect(text(tree.root)).toContain('放弃未保存的更改')
})
it('keeps explicit save on the existing Store CAS and identity adapter after applying parameters',async()=>{
  withProxyOptions();await mount();fillParameters()
  act(()=>submitParameters());expectNoProxyEffects()
  await act(async()=>choose());await act(async()=>button('保存配置').props.onClick())
  expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
  const [id,version,raw,revision,epoch]=native.StoreEnvironmentSave.mock.calls[0]
  expect([id,version,revision,epoch]).toEqual(['shop-a','4',7,generation])
  expect(JSON.parse(raw)).toMatchObject({proxyId:'',proxyConfig:'http://fixture-user:fixture-secret@proxy.example.invalid:8080'})
  expect(native.StoreProxyCheck).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it('cancels parameter entry without touching the saved network or dirty state and clears credentials on reopening',async()=>{
  await mount();fillParameters();act(()=>button('取消').props.onClick())
  expect(field('本地代理地址').props.value).toBe(input.proxyConfig)
  act(()=>button('填写代理参数').props.onClick());expect(field('代理账号').props.value).toBe('');expect(field('代理密码').props.value).toBe('')
  act(()=>button('取消').props.onClick());act(()=>button('返回店铺').props.onClick())
  expect(location.pathname).toBe('/stores/shop-a');expectNoProxyEffects()
})
it('shows a fixed error for invalid parameters and preserves the existing draft',async()=>{
  await mount();fillParameters({'代理地址':'fixture-user:fixture-secret@proxy.example.invalid'});act(()=>submitParameters())
  expect(text(tree.root)).toContain('代理地址只填写域名或 IP');expect(text(tree.root)).not.toContain('fixture-secret')
  expect(field('本地代理地址').props.value).toBe(input.proxyConfig);expectNoProxyEffects()
})
it('invalidates captured parameter callbacks when the underlying URI is edited',async()=>{
  await mount();fillParameters();const apply=parameterForm().props.onSubmit
  act(()=>field('本地代理地址').props.onChange({target:{value:'https://new.example.invalid:8443'}}))
  act(()=>apply({preventDefault(){}}));expect(field('本地代理地址').props.value).toBe('https://new.example.invalid:8443')
  act(()=>button('填写代理参数').props.onClick());expect(field('代理密码').props.value).toBe('');expectNoProxyEffects()
})
it('discards the old Store parameter form and ignores its callback after navigation',async()=>{
  native.StoreEnvironmentOptions.mockImplementation(id=>Promise.resolve(JSON.stringify({input:{...input,profileName:id,proxyConfig:id==='shop-a'?input.proxyConfig:'http://other.example.invalid:2080'},cores:[],binding:{...state.binding,bindingVersion:id==='shop-a'?'4':'19'}})))
  await mount();fillParameters();const apply=parameterForm().props.onSubmit
  await act(async()=>navigate('/stores/shop-b/environment/edit'))
  act(()=>apply({preventDefault(){}}));expect(field('本地代理地址').props.value).toBe('http://other.example.invalid:2080')
  act(()=>button('填写代理参数').props.onClick());expect(field('代理账号').props.value).toBe('');expect(field('代理密码').props.value).toBe('')
  act(()=>button('取消').props.onClick());expectNoProxyEffects()
  await act(async()=>choose());await act(async()=>button('保存配置').props.onClick())
  expect(native.StoreEnvironmentSave.mock.calls[0][0]).toBe('shop-b');expect(native.StoreEnvironmentSave.mock.calls[0][1]).toBe('19')
})
it('clears unapplied credentials on enterprise switch and keeps the new revision',async()=>{
  const orgB={id:'org-b',name:'Org B',permissions:['workbench.store.browser.use']}
  native.StoreWorkbenchContext.mockResolvedValue(JSON.stringify({...context,context:{...context.context,organizations:[...context.context.organizations,orgB]}}))
  native.StoreWorkbenchSelectOrganization=vi.fn().mockResolvedValue(JSON.stringify({...context,revision:8,context:{...context.context,effectiveOrganizationId:'org-b',organizations:[orgB]}}))
  await mount();fillParameters();const apply=parameterForm().props.onSubmit
  await act(async()=>field('当前企业').props.onChange({target:{value:'org-b'}}))
  act(()=>apply({preventDefault(){}}));expect(location.pathname).toBe('/stores');expectNoProxyEffects()
  await act(async()=>navigate('/stores/shop-a/environment/edit'))
  act(()=>button('填写代理参数').props.onClick());expect(field('代理密码').props.value).toBe('')
  expect(native.StoreEnvironmentOptions.mock.calls.at(-1)).toEqual(['shop-a',8,generation])
})
it('drops credentials immediately on authentication replacement and ignores its captured submit',async()=>{
  await mount();fillParameters();const apply=parameterForm().props.onSubmit
  act(()=>{acceptAuth({generation:generation+1,state:'signed_in',configured:true,subject:'replacement',expiresAt:new Date(Date.now()+60000).toISOString()},beginAuthAction());apply({preventDefault(){}})})
  expect(tree.root.findAllByProps({'aria-label':'自有代理参数'})).toHaveLength(0)
  expect(field('本地代理地址').props.value).toBe(input.proxyConfig);expectNoProxyEffects()
})
it('keeps parameter entry disabled if Store options are unauthorized or unavailable',async()=>{
  native.StoreEnvironmentOptions.mockRejectedValue(new Error('PERMISSION_DENIED'))
  await mount();expect(button('填写代理参数').props.disabled).toBe(true)
  act(()=>button('填写代理参数').props.onClick());expect(tree.root.findAllByProps({'aria-label':'自有代理参数'})).toHaveLength(0)
  expectNoProxyEffects()
})
it('does not apply a captured parameter form during an explicit save or after unmount',async()=>{
  const pending=deferred();native.StoreEnvironmentSave.mockReturnValue(pending.promise)
  await mount();await act(async()=>choose());fillParameters();const apply=parameterForm().props.onSubmit
  await act(async()=>{button('保存配置').props.onClick();apply({preventDefault(){}})})
  expect(field('本地代理地址').props.value).toBe(input.proxyConfig);expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1)
  act(()=>{tree.unmount();tree=null;apply({preventDefault(){}})});await act(async()=>pending.resolve(JSON.stringify(state)))
  expect(native.StoreEnvironmentSave).toHaveBeenCalledTimes(1);expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
})
it('invalidates an existing check when parameters are applied without starting another check',async()=>{
  withProxyOptions();const pending=deferred();native.StoreProxyCheck.mockReturnValue(pending.promise)
  await mount();act(()=>field('检查目标').props.onChange({target:{value:'fixture'}}))
  await act(async()=>button('检查此代理').props.onClick())
  const oldRequest=JSON.parse(native.StoreProxyCheck.mock.calls[0][2])
  fillParameters();await act(async()=>submitParameters())
  expect(native.StoreProxyCheck).toHaveBeenCalledTimes(1)
  expect(native.StoreProxyCheckCancel).toHaveBeenCalledTimes(1)
  expect(button('检查此代理').props.disabled).toBe(true)
  expect(text(tree.root)).toContain('原检查结果已失效')
  await act(async()=>pending.resolve(JSON.stringify(proxyResult(oldRequest))))
  expect(text(tree.root)).not.toContain('代理服务器 TCP')
  expect(button('检查此代理').props.disabled).toBe(false)
  expect(native.StoreEnvironmentSave).not.toHaveBeenCalled();expect(native.StoreEnvironmentOpen).not.toHaveBeenCalled()
  expect(native.StoreProxyCheck).toHaveBeenCalledTimes(1)
})
