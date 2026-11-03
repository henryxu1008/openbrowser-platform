// Offline entry verification: production gate/layout/routes/adapters, synthetic
// native responses only. No real authentication or business action is available.
import React,{Suspense} from 'react'
import {createRoot} from 'react-dom/client'
import {MemoryRouter,useLocation} from 'react-router-dom'
import {AuthGate} from '../../src/auth/AuthGate'
import {acceptAuth,authSnapshot,beginAuthAction} from '../../src/auth/session'
import {ThemeProvider} from '../../src/shared/theme'
import {Layout} from '../../src/shared/layout'
import {AppRoutes} from '../../src/routes/AppRoutes'
import '../../src/index.css'

const params=new URLSearchParams(location.search),calls=[]
const status={generation:authSnapshot().generation+1,state:params.get('auth')==='signed-out'?'signed_out':'signed_in',configured:true,subject:'离线示例用户',expiresAt:new Date(Date.now()+3600000).toISOString()}
let org=params.get('selected')==='1'?'fixture-org-a':'',revision=7
const json=value=>Promise.resolve(JSON.stringify(value))
const context=()=>({revision,context:{user:{id:'synthetic'},effectiveOrganizationId:org||null,selectionRequired:!org,organizations:['fixture-org-a','fixture-org-b'].map((id,i)=>({id,name:`示例企业 ${i+1}`,permissions:['workbench.store.browser.use']}))}})
const store={id:'fixture-store',name:'示例店铺',platform:'temu',region:'US',recordStatus:'active'}
const native={
 AuthStatus:()=>Promise.resolve(status),GetAppConfig:()=>Promise.resolve({version:'offline-fixture'}),
 StoreWorkbenchContext:()=>params.get('deny')==='1'?Promise.reject(new Error('PERMISSION_DENIED')):json(context()),
 StoreWorkbenchSelectOrganization:id=>{org=id;revision++;return json(context())},
 StoreWorkbenchList:()=>json({items:[store],pagination:{page:1,pageSize:20,total:1}}),
 BrowserProfileList:()=>Promise.resolve([]),BrowserProxyList:()=>Promise.resolve([]),ListGroups:()=>Promise.resolve([]),BrowserCoreList:()=>Promise.resolve([]),
}
window.go={main:{App:new Proxy(native,{get:(target,key)=>(...args)=>{
 calls.push({method:String(key),args})
 if(!(key in target))return Promise.reject(new Error(`Offline fixture rejects native method: ${String(key)}`))
 return target[key](...args)
}})}}
window.runtime={EventsOnMultiple:()=>()=>{}}
window.fetch=()=>Promise.reject(new Error('Offline fixture rejects fetch'))
acceptAuth(status,beginAuthAction())
function Trace(){const route=useLocation();return <details style={{position:'fixed',bottom:0,right:0,zIndex:10000,padding:4,fontSize:11,background:'#fff',color:'#475569',border:'1px solid #cbd5e1'}}>
 <summary data-testid="fixture-route">离线合成夹具 · {route.pathname}{route.search}</summary>
 <pre data-testid="fixture-calls" style={{maxHeight:240,maxWidth:600,overflow:'auto'}}>{JSON.stringify(calls,null,2)}</pre>
 </details>}
createRoot(document.getElementById('root')).render(<ThemeProvider defaultTheme="light"><MemoryRouter initialEntries={[params.get('path')||'/']}>
 <AuthGate><Layout><Suspense fallback={<p>页面加载中…</p>}><AppRoutes/></Suspense></Layout></AuthGate><Trace/>
</MemoryRouter></ThemeProvider>)
