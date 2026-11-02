import type { StoreProxyOptions, storeAPI } from './api'
import type { CheckScope, ProxyCheckPort } from './proxyCheck'

// Retains the captured Store API/auth epoch, revision and binding. No generic
// proxy method, bearer token, caller URL/header or mutable global setting enters.
export function storeProxyPort(api: ReturnType<typeof storeAPI>, scope: CheckScope, options: StoreProxyOptions): ProxyCheckPort {
  const captured={...scope},catalogVersion=options.catalogVersion
  const checkProxy=api.checkProxy,cancelProxyCheck=api.cancelProxyCheck
  const candidates=Array.isArray(options.targets)?options.targets:[]
  const seen=new Set<string>()
  const valid=candidates.length<=8&&candidates.every(t=>{
    try {if(!t||typeof t.id!=='string'||typeof t.label!=='string'||typeof t.url!=='string'||seen.has(t.id))return false
      const u=new URL(t.url);seen.add(t.id)
      return /^[a-z0-9][a-z0-9_-]{0,63}$/.test(t.id)&&t.label.length>0&&t.label.length<=120&&t.url.length<=2048&&u.protocol==='https:'&&!u.username&&!u.password&&!u.search&&!u.hash
    } catch{return false}
  })
  const targets=valid?candidates.map(({id,label,url})=>Object.freeze({id,label,url})):[]
  const available=options.available&&valid&&targets.length>0&&typeof catalogVersion==='string'&&/^[0-9a-f]{64}$/.test(catalogVersion)
  return {
    targets: available?targets:[],
    ...(!available ? {unavailableReason:'TARGET_CONFIGURATION_REQUIRED' as const}:{}),
    async check(request,signal) {
      const {requestId,proxyConfig,targetId}=request
      if (signal.aborted) throw new Error('store_proxy_check_canceled')
      if(!available||!targets.some(t=>t.id===targetId))throw new Error('store_proxy_check_targets_required')
      const pending=checkProxy(captured.store,captured.bindingVersion,{requestId,proxyConfig,targetId,catalogVersion,expiresAtUnixMs:Date.now()+12000},captured.revision)
      const cancel=()=>{ void cancelProxyCheck(captured.store,requestId,captured.revision).catch(()=>{}) }
      signal.addEventListener('abort',cancel,{once:true})
      if(signal.aborted)cancel()
      try { return await pending } finally {signal.removeEventListener('abort',cancel)}
    },
  }
}
