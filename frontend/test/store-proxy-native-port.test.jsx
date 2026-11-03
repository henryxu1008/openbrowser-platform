import {expect,it,vi} from 'vitest'
import {storeProxyPort} from '../src/modules/stores/proxyCheckPort'
const options=()=>({available:true,reason:'NONE',catalogVersion:'c'.repeat(64),targets:[{id:'fixture',label:'Offline target',url:'https://target.example.invalid/check'}]})
const scope=()=>({identity:'fixture-member',generation:7,organization:'org',revision:4,store:'shop',bindingVersion:'2',connector:'native'})
const request=()=>({requestId:'fixture-id',proxyConfig:'http://fixture-user:fixture-secret@proxy.example.invalid:8080',targetId:'fixture',scope:{store:'crafted-other'}})
it('pins original scope, catalog and cancellation key rather than adopting caller data',async()=>{
 let resolve;const pending=new Promise(r=>{resolve=r});const api={checkProxy:vi.fn(()=>pending),cancelProxyCheck:vi.fn().mockResolvedValue({settled:false})},s=scope(),o=options()
 const port=storeProxyPort(api,s,o),r=request(),abort=new AbortController();s.store='replacement';s.bindingVersion='99';o.catalogVersion='x';o.targets[0].url='https://other.example.invalid/'
 const run=port.check(r,abort.signal);r.requestId='replaced-id';abort.abort()
 expect(api.checkProxy).toHaveBeenCalledWith('shop','2',{requestId:'fixture-id',proxyConfig:expect.any(String),targetId:'fixture',catalogVersion:'c'.repeat(64),expiresAtUnixMs:expect.any(Number)},4)
 expect(api.cancelProxyCheck).toHaveBeenCalledWith('shop','fixture-id',4)
 expect(port.targets[0].url).toBe('https://target.example.invalid/check')
 resolve({});await run
})
it.each([null,{id:'fixture',label:'x',url:'https://user:fixture-secret@target.example.invalid/'},{id:'fixture',label:'x',url:'http://target.example.invalid/'}])('fails closed for malformed target metadata without displaying credentials',async target=>{
 const api={checkProxy:vi.fn(),cancelProxyCheck:vi.fn()},port=storeProxyPort(api,scope(),{...options(),targets:[target]})
 expect(port.targets).toEqual([]);expect(port.unavailableReason).toBe('TARGET_CONFIGURATION_REQUIRED')
 await expect(port.check(request(),new AbortController().signal)).rejects.toThrow('store_proxy_check_targets_required');expect(api.checkProxy).not.toHaveBeenCalled()
})
it('rejects an already canceled action before crossing native bridge',async()=>{
 const api={checkProxy:vi.fn(),cancelProxyCheck:vi.fn()},port=storeProxyPort(api,scope(),options()),abort=new AbortController();abort.abort()
 await expect(port.check(request(),abort.signal)).rejects.toThrow('store_proxy_check_canceled');expect(api.checkProxy).not.toHaveBeenCalled()
})
