// Offline visual/keyboard fixture only. Production components, adapters and
// generated bridge remain intact; all native calls terminate in these mocks.
import React, {useState} from 'react'
import {createRoot} from 'react-dom/client'
import {MemoryRouter, Route, Routes, useLocation, useNavigate} from 'react-router-dom'
import {StoreWorkspace, StoreListPage, StoreDetailPage, StoreEnvironmentEditPage} from '../../src/modules/stores/StorePages'
import {Button, Modal} from '../../src/shared/components'
import {acceptAuth, authSnapshot, beginAuthAction} from '../../src/auth/session'
import '../../src/index.css'

const calls = []
const releaseId = '10000000-0000-0000-0000-000000000001'
const operationId = '10000000-0000-0000-0000-000000000099'
const binding = {bindingId:'fixture-binding',profileId:'fixture-profile',bindingVersion:'4'}
const input = {profileName:'示例店铺',userDataDir:'',coreId:releaseId,restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'',memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
const environment = {binding,name:'示例店铺',coreId:releaseId,proxySummary:'直连',running:false,runtimeKnown:true,pending:null,lastOperation:{operationId,action:'open',phase:'done',outcome:'failed',reasonCode:'fixture_only',reportState:'confirmed'},reports:[],observedAt:'2026-10-31T10:40:35Z'}
const store = {id:'fixture-store',name:'示例店铺',platform:'temu',region:'US',recordStatus:'active'}
const record = {eventId:'fixture-event',operationId,actorSubject:'示例成员',memberRef:'fixture-member',action:'browser_open_reported',serverTime:'2026-10-31T10:40:35Z',clientObservedAt:'2026-10-31T10:40:34Z',outcome:'failed',reasonCode:'fixture_only',profileRef:'fixture-profile',bindingRef:'fixture-binding',factSource:'client_report'}
let org='fixture-org-a', revision=7
const context = () => ({revision,context:{user:{id:'offline-fixture'},effectiveOrganizationId:org,selectionRequired:false,organizations:['fixture-org-a','fixture-org-b'].map((id,i)=>({id,name:`示例企业 ${i+1}`,permissions:['workbench.store.browser.use']}))}})
const json = value => Promise.resolve(JSON.stringify(value))
const native = {
  StoreWorkbenchContext:()=>json(context()),
  StoreWorkbenchSelectOrganization:id=>{org=id;revision++;return json(context())},
  StoreWorkbenchList:()=>json({items:[store],pagination:{page:1,pageSize:20,total:1}}),
  StoreWorkbenchGet:()=>json(store), StoreEnvironmentGet:()=>json(environment),
  StoreEnvironmentOptions:()=>json({input,binding,cores:[]}),
  StoreAuditList:()=>json({items:[record],nextCursor:null,coverage:[],canFilterActor:false}),
  StoreOperationGet:()=>json({operationId,outcome:'failed',reportState:'confirmed',reasonCode:'fixture_only',detail:'离线示例回执；没有执行浏览器操作。'}),
  BrowserKernelCatalog:()=>json([{releaseId,version:'130.0.0.1',os:'windows',arch:'amd64',recommended:true,installed:true}]),
  // A save error exercises the existing error dialog without writing anything.
  StoreEnvironmentSave:()=>Promise.reject(new Error('BINDING_VERSION_CONFLICT')),
}
window.go={main:{App:new Proxy(native,{get:(target,key)=>{
  if(!(key in target))throw new Error(`Offline fixture rejects native method: ${String(key)}`)
  return (...args)=>{calls.push({method:key,args});return target[key](...args)}
}})}}
window.runtime={EventsOnMultiple:()=>()=>{}}
window.fetch=()=>Promise.reject(new Error('Offline fixture rejects fetch'))
acceptAuth({generation:authSnapshot().generation+1,state:'signed_in',configured:true,expiresAt:new Date(Date.now()+3600000).toISOString()},beginAuthAction())

function Fixture(){
  const navigate=useNavigate(),location=useLocation(),[plain,setPlain]=useState(false),[theme,setTheme]=useState('light'),[,refresh]=useState(0)
  return <>
    <nav aria-label="离线验收工具" style={{padding:16,display:'flex',gap:12,flexWrap:'wrap'}}>
      <Button onClick={()=>navigate('/stores/fixture-store/environment/edit')}>配置示例</Button>
      <Button onClick={()=>navigate('/stores/fixture-store?tab=records')}>记录示例</Button>
      <Button onClick={()=>navigate('/stores/fixture-store')}>回执示例</Button>
      <Button variant="secondary" onClick={()=>setPlain(true)}>普通弹窗对照</Button>
      <label>主题 <select aria-label="验收主题" value={theme} onChange={e=>{setTheme(e.target.value);document.documentElement.dataset.theme=e.target.value}}>{['light','dark','cream','mint','ocean'].map(value=><option key={value}>{value}</option>)}</select></label>
      <Button variant="secondary" onClick={()=>refresh(n=>n+1)}>读取调用记录</Button>
    </nav>
    <p style={{padding:'0 16px'}}>离线组件验收 · 所有数据为合成数据 · 路由：{location.pathname}</p>
    <main style={{padding:24}}><Routes><Route path="/stores" element={<StoreWorkspace/>}>
      <Route index element={<StoreListPage/>}/><Route path=":storeId" element={<StoreDetailPage/>}/><Route path=":storeId/environment/edit" element={<StoreEnvironmentEditPage/>}/>
    </Route></Routes></main>
    <Modal open={plain} title="普通弹窗对照" onClose={()=>setPlain(false)} footer={<Button disabled>禁用按钮</Button>}><p>其他页面继续使用原主题。</p></Modal>
    <details style={{padding:16}}><summary>Mock bridge 调用记录</summary><pre data-testid="bridge-calls" style={{whiteSpace:'pre-wrap',wordBreak:'break-all'}}>{JSON.stringify(calls,null,2)}</pre></details>
  </>
}
createRoot(document.getElementById('root')).render(<MemoryRouter initialEntries={['/stores/fixture-store/environment/edit']}><Fixture/></MemoryRouter>)
