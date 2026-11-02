// Synthetic contract fixture. No fetch, TCP, credentials, connector or product execution.
import React,{StrictMode,useMemo,useRef,useState} from 'react'
import {createRoot} from 'react-dom/client'
import {MemoryRouter} from 'react-router-dom'
import {BrowserEditPage} from '../../src/modules/browser/pages/BrowserEditPage'
import {Button} from '../../src/shared/components'
import {acceptAuth,authSnapshot,beginAuthAction} from '../../src/auth/session'
import '../../src/index.css'
import '../../src/modules/stores/store-workbench.css'

const releaseId='10000000-0000-0000-0000-000000000001'
const counts={checks:0,saves:0,back:0,unexpected:0}
window.go={main:{App:new Proxy({BrowserKernelCatalog:()=>Promise.resolve(JSON.stringify([
  {releaseId,version:'130.0.0.1',os:'windows',arch:'amd64',recommended:true,installed:true},
]))},{get:(target,key)=>{if(key in target)return target[key];return ()=>{counts.unexpected++;throw new Error('Offline fixture rejects native operation')}}})}}
window.runtime={EventsOnMultiple:()=>()=>{}}
window.fetch=()=>Promise.reject(new Error('Offline fixture rejects fetch'))
acceptAuth({generation:authSnapshot().generation+1,state:'signed_in',configured:true,expiresAt:new Date(Date.now()+3600000).toISOString()},beginAuthAction())
const targets=[{id:'fixture-target',label:'离线示例目标',url:'https://target.example.invalid/check'}]
const input={profileName:'示例店铺',userDataDir:'',coreId:releaseId,restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'socks5://proxy.example.invalid:1080',memoryLimitMb:512,launchArgs:[],tags:[],keywords:[],groupId:''}
function Fixture(){
  const [theme,setTheme]=useState('light'),[revision,setRevision]=useState(1),[mode,setMode]=useState('success'),[,refresh]=useState(0)
  const modeRef=useRef(mode);modeRef.current=mode
  const port=useMemo(()=>({targets,check:(request,signal)=>{
    counts.checks++;refresh(n=>n+1)
    const result={schema:'store-proxy-check-v1',requestId:request.requestId,targetId:request.targetId,syntax:'passed',tcp:'passed',target:'passed',reason:'NONE'}
    const selected=modeRef.current
    return new Promise(resolve=>{
      const timer=setTimeout(()=>{
        if(selected==='target_failed')resolve({...result,target:'failed',reason:'TARGET_UNREACHABLE',recentLogs:['fixture-secret']})
        else if(selected==='tcp_only')resolve({...result,target:'not_checked',reason:'UNAVAILABLE'})
        else resolve(result)
      },selected==='pending'?20000:250)
      signal.addEventListener('abort',()=>{clearTimeout(timer);setTimeout(()=>resolve({...result,target:'not_checked',reason:'CANCELED'}),400)},{once:true})
    })
  }}),[])
  const managed=useMemo(()=>({title:'店铺环境配置',load:async()=>({input,cores:[]}),save:async()=>{counts.saves++;refresh(n=>n+1)},back:()=>{counts.back++;refresh(n=>n+1)},
    proxyCheck:{port,scope:{identity:'fixture-member',generation:1,organization:'fixture-org',revision,store:'fixture-store',bindingVersion:'2',connector:'xray'}}}),[port,revision])
  window.proxyFixture={counts,changeScope:()=>setRevision(n=>n+1)}
  return <>
    <div className="p-4 space-y-3">
      <p>离线合同验收 · 合成数据与模拟结果 · 真实连接功能未接入</p>
      <div className="flex flex-wrap gap-3">
        <label>主题 <select aria-label="验收主题" value={theme} onChange={e=>{setTheme(e.target.value);document.documentElement.dataset.theme=e.target.value}}><option>light</option><option>dark</option></select></label>
        <label>模拟结果 <select aria-label="模拟结果" value={mode} onChange={e=>setMode(e.target.value)}><option value="success">三项通过</option><option value="target_failed">目标不可达</option><option value="tcp_only">仅 TCP 通过</option><option value="pending">等待 / 取消 / 超时</option></select></label>
        <Button type="button" variant="secondary" size="sm" onClick={()=>setRevision(n=>n+1)}>切换企业上下文</Button>
      </div>
      <output aria-label="模拟调用计数">检查 {counts.checks} · 保存 {counts.saves} · 返回 {counts.back} · 意外原生调用 {counts.unexpected}</output>
    </div>
    <main className="store-workbench p-4 max-w-5xl mx-auto"><BrowserEditPage managed={managed}/></main>
  </>
}
createRoot(document.getElementById('root')).render(<StrictMode><MemoryRouter><Fixture/></MemoryRouter></StrictMode>)
