// Actual Store pages/auth capture/generated bridge, synthetic native endpoints.
// No real account, proxy, save or browser process is used.
import React,{StrictMode,useState} from 'react'
import {createRoot} from 'react-dom/client'
import {MemoryRouter,Route,Routes} from 'react-router-dom'
import {StoreWorkspace,StoreListPage,StoreEnvironmentEditPage} from '../../src/modules/stores/StorePages'
import {acceptAuth,authSnapshot,beginAuthAction} from '../../src/auth/session'
import '../../src/index.css'
const params=new URLSearchParams(location.search),configured=params.get('catalog')==='fixture'
const storeID='10000000-0000-0000-0000-000000000010',releaseID='10000000-0000-0000-0000-000000000001'
const counts={checks:0,probes:0,cancels:0,saves:0,opens:0,unexpected:0}
const binding={bindingId:'fixture-binding',profileId:'fixture-profile',bindingVersion:'4'}
const input={profileName:'示例店铺',userDataDir:'',coreId:releaseID,restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'socks5://proxy.example.invalid:1080',memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
let org='fixture-org-a',revision=1,mode='success',pending=null,refresh=()=>{}
const context=()=>({revision,context:{user:{id:'fixture-member'},effectiveOrganizationId:org,selectionRequired:false,organizations:['fixture-org-a','fixture-org-b'].map((id,i)=>({id,name:`示例企业 ${i+1}`,permissions:['workbench.store.browser.use']}))}})
const json=value=>Promise.resolve(JSON.stringify(value))
const native={
 StoreWorkbenchContext:()=>json(context()),StoreWorkbenchSelectOrganization:id=>{org=id;revision++;return json(context())},
 StoreWorkbenchList:()=>json({items:[],pagination:{page:1,pageSize:20,total:0}}),
 StoreEnvironmentOptions:()=>json({input,cores:[],binding,proxyCheck:{available:configured,reason:configured?'NONE':'TARGET_CONFIGURATION_REQUIRED',catalogVersion:configured?'c'.repeat(64):'',targets:configured?[{id:'fixture',label:'离线示例目标',url:'https://target.example.invalid/check'}]:[]}}),
 BrowserKernelCatalog:()=>json([{releaseId:releaseID,version:'130.0.0.1',os:'windows',arch:'amd64',recommended:true,installed:true}]),
 StoreProxyCheck:(id,version,raw)=>{
  counts.checks++;const request=JSON.parse(raw)
  const result={schema:'store-proxy-check-v1',requestId:request.requestId,targetId:request.targetId,syntax:'not_checked',tcp:'not_checked',target:'not_checked',reason:'CHECK_BUSY'}
  if(pending){refresh();return json(result)}
  counts.probes++;refresh()
  return new Promise(resolve=>{
   const finish=canceled=>{if(!pending)return;clearTimeout(pending.timer);pending=null;resolve(JSON.stringify(canceled?{...result,reason:'CANCELED'}:{...result,syntax:'passed',tcp:'passed',target:mode==='tcp_only'?'not_checked':'passed',reason:mode==='tcp_only'?'UNAVAILABLE':'NONE'}));refresh()}
   pending={requestId:request.requestId,finish,timer:setTimeout(()=>finish(false),mode==='pending'?20000:250)}
  })
 },
 StoreProxyCheckCancel:(id,key)=>{counts.cancels++;refresh();return new Promise(resolve=>setTimeout(()=>{if(pending?.requestId===key)pending.finish(true);resolve(JSON.stringify({requestId:key,settled:true}))},400))},
 StoreEnvironmentSave:()=>{counts.saves++;refresh();throw new Error('Offline fixture rejects save')},
 StoreEnvironmentOpen:()=>{counts.opens++;refresh();throw new Error('Offline fixture rejects open')},
}
window.go={main:{App:new Proxy(native,{get:(target,key)=>{if(key in target)return target[key];return ()=>{counts.unexpected++;refresh();throw new Error('Offline fixture rejects native method')}}})}}
window.runtime={EventsOnMultiple:()=>()=>{}}
window.fetch=()=>Promise.reject(new Error('Offline fixture rejects fetch'))
acceptAuth({generation:authSnapshot().generation+1,state:'signed_in',configured:true,expiresAt:new Date(Date.now()+3600000).toISOString()},beginAuthAction())
function Fixture(){
 const [theme,setTheme]=useState('light'),[,setTick]=useState(0);refresh=()=>setTick(n=>n+1)
 window.proxyFixture={counts,changeScope:()=>{const select=document.querySelector('select[aria-label="当前企业"]');Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,'value').set.call(select,'fixture-org-b');select.dispatchEvent(new Event('change',{bubbles:true}))}}
 return <>
  <div className="p-4 space-y-3"><p>离线原生合同验收 · 合成账号与模拟服务 · 未执行真实代理检查</p>
   <label>主题 <select aria-label="验收主题" value={theme} onChange={e=>{setTheme(e.target.value);document.documentElement.dataset.theme=e.target.value}}><option>light</option><option>dark</option></select></label>
   <label>模拟结果 <select aria-label="模拟结果" defaultValue="success" onChange={e=>{mode=e.target.value}}><option value="success">三项通过</option><option value="tcp_only">仅 TCP 通过</option><option value="pending">等待 / 取消 / 超时</option></select></label>
   <p>目录：{configured?'仅本 fixture 注入的合成目录':'生产默认：未配置发布方目标'}</p>
   <output>检查 {counts.checks} · 模拟探测 {counts.probes} · 取消 {counts.cancels} · 保存 {counts.saves} · 打开 {counts.opens} · 意外原生调用 {counts.unexpected}</output>
  </div>
  <main className="p-4 max-w-5xl mx-auto"><Routes><Route path="/stores" element={<StoreWorkspace/>}><Route index element={<StoreListPage/>}/><Route path=":storeId/environment/edit" element={<StoreEnvironmentEditPage/>}/></Route></Routes></main>
 </>
}
createRoot(document.getElementById('root')).render(<StrictMode><MemoryRouter initialEntries={[`/stores/${storeID}/environment/edit`]}><Fixture/></MemoryRouter></StrictMode>)
