import { afterEach, describe, expect, it, vi } from 'vitest'
import { createProxyCheckController, inspectProxyInput, safeCheckResult } from '../src/modules/stores/proxyCheck'

const scope = { identity: 'fixture-member', generation: 7, organization: 'fixture-org', revision: 4,
  store: 'fixture-store', bindingVersion: '2', connector: 'xray' }
const targets = [{ id: 'fixture-target', label: '离线示例目标', url: 'https://target.example.invalid/check' }]
const deferred = () => { let resolve, reject; const promise = new Promise((a,b) => { resolve=a; reject=b }); return { promise, resolve, reject } }
const result = (request, overrides={}) => ({ schema:'store-proxy-check-v1', requestId:request.requestId,
  targetId:request.targetId, syntax:'passed', tcp:'passed', target:'passed', reason:'NONE', ...overrides })
function setup(overrides={}) {
  let current = { scope:{...scope}, proxyConfig:'socks5://fixture-user:fixture-secret@proxy.example.invalid:1080', enabled:true }
  const pending=deferred(), check=vi.fn(()=>pending.promise)
  const controller=createProxyCheckController({targets,check},()=>current,{id:()=> 'fixture-request',...overrides})
  return {controller,pending,check,current:()=>current,change:v=>{current={...current,...v}}}
}
afterEach(()=>vi.useRealTimers())

describe('bounded input and evidence',()=>{
  it.each(['','   ','direct://','socks5://host','http://host:0','http://host:65536','http://host/path',
    'http://host?secret=value','http://host/#fragment','http://host\n','http://[bad','http:///host','http://'+ '用'.repeat(2800)+'@host:80','a'.repeat(8193)])('rejects illegal proxy input without transport: %s',async value=>{
    // Trailing whitespace is allowed after trim; embedded controls are forbidden.
    const x=setup();x.change({proxyConfig:value==='http://host\n'?'http://ho\nst':value})
    await x.controller.start('fixture-target');expect(x.check).not.toHaveBeenCalled()
    expect(x.controller.snapshot().reason).toMatch(/INVALID_PROXY|UNSUPPORTED_PROTOCOL/)
  })
  it.each(['http://proxy.example.invalid:8080','https://proxy.example.invalid',
    'socks5://user:p%40ss@[::1]:1080',' HTTP://proxy.example.invalid:8080 '])('accepts supported syntax without a network request: %s',value=>{
    expect(inspectProxyInput(value)).toBe(null)
  })
  it.each(['vless://fixture','hysteria2://fixture','tuic://fixture','anytls://fixture','ss://fixture','mihomo://fixture'])('does not switch stacks for %s',async value=>{
    const x=setup();x.change({proxyConfig:value});await x.controller.start('fixture-target')
    expect(x.check).not.toHaveBeenCalled();expect(x.controller.snapshot().reason).toBe('UNSUPPORTED_PROTOCOL')
  })
  it('requires explicit known single target and enabled owning context',async()=>{
    const x=setup();await x.controller.start('');await x.controller.start('other');expect(x.check).not.toHaveBeenCalled()
    x.change({enabled:false});await x.controller.start('fixture-target');expect(x.check).not.toHaveBeenCalled()
  })
  it('copies only allowlisted evidence and never diagnostics, credential errors or logs',async()=>{
    const x=setup(),run=x.controller.start('fixture-target'),request=x.check.mock.calls[0][0]
    x.pending.resolve(result(request,{recentLogs:['fixture-secret'],error:'fixture-secret',proxyConfig:request.proxyConfig,ip:'203.0.113.5',country:'US'}));await run
    const view=x.controller.snapshot();expect(view.result).toEqual(result(request))
    expect(JSON.stringify(view)).not.toMatch(/fixture-secret|recentLogs|203\.0\.113|country|proxyConfig/)
  })
  it('keeps TCP success distinct from unproven target access',async()=>{
    const x=setup(),run=x.controller.start('fixture-target');x.pending.resolve(result(x.check.mock.calls[0][0],{target:'failed',reason:'TARGET_UNREACHABLE'}));await run
    expect(x.controller.snapshot().result).toMatchObject({tcp:'passed',target:'failed',reason:'TARGET_UNREACHABLE'})
  })
  it.each([
    {schema:'diagnostic-v2'}, {requestId:'other'}, {targetId:'other'}, {syntax:'maybe'}, {reason:'raw fixture-secret'},
    {syntax:'failed'}, {tcp:'not_checked'}, {target:'not_checked'}, {reason:'TIMEOUT'},
    {syntax:{toString:()=> 'passed'}}, {target:'not_checked',reason:'PERMISSION_DENIED'},
    {target:'not_checked',tcp:'failed',reason:'CONNECTOR_REQUIRED'},
  ])('rejects malformed, mismatched or contradictory result %j',overrides=>{
    const request={requestId:'fixture-request',targetId:'fixture-target'}
    expect(safeCheckResult(result(request,overrides),request)).toBe(null)
  })
  it('maps credential-bearing thrown errors to a fixed unavailable code',async()=>{
    const x=setup(),run=x.controller.start('fixture-target');x.pending.reject(new Error(x.current().proxyConfig));await run
    expect(x.controller.snapshot()).toEqual({phase:'failed',busy:false,reason:'UNAVAILABLE'})
  })
})

describe('operation ownership and lifetime',()=>{
  it('deduplicates pending work and sends exactly one unchanged connector and explicit target',async()=>{
    const x=setup(),run=x.controller.start('fixture-target');await x.controller.start('fixture-target')
    expect(x.check).toHaveBeenCalledTimes(1)
    expect(x.check.mock.calls[0][0]).toEqual({requestId:'fixture-request',scope,proxyConfig:x.current().proxyConfig,targetId:'fixture-target'})
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run
    expect(x.controller.snapshot().result.target).toBe('passed')
  })
  it('cancels without treating cancellation as worker settlement or allowing duplicate retry',async()=>{
    const x=setup(),run=x.controller.start('fixture-target'),signal=x.check.mock.calls[0][1]
    x.controller.cancel();expect(signal.aborted).toBe(true);expect(x.controller.snapshot()).toMatchObject({phase:'canceled',busy:true})
    await x.controller.start('fixture-target');expect(x.check).toHaveBeenCalledTimes(1)
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run
    expect(x.controller.snapshot()).toEqual({phase:'canceled',busy:false,reason:'CANCELED',result:undefined})
  })
  it('bounds deadline and ignores late success while retaining the retry lock until settlement',async()=>{
    vi.useFakeTimers();const x=setup({timeoutMs:100000}),run=x.controller.start('fixture-target')
    await vi.advanceTimersByTimeAsync(12000);expect(x.check.mock.calls[0][1].aborted).toBe(true)
    expect(x.controller.snapshot()).toMatchObject({phase:'timed_out',busy:true,reason:'TIMEOUT'})
    await x.controller.start('fixture-target');expect(x.check).toHaveBeenCalledTimes(1)
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run
    expect(x.controller.snapshot()).toMatchObject({phase:'timed_out',busy:false,reason:'TIMEOUT'})
    expect(x.controller.snapshot().result).toBeUndefined()
  })
  it.each([['identity','other-member'],['generation',8],['organization','other-org'],['revision',5],
    ['store','other-store'],['bindingVersion','3'],['connector','mihomo']])('invalidates and cancels on %s change',async(field,value)=>{
    const x=setup(),run=x.controller.start('fixture-target')
    x.change({scope:{...scope,[field]:value}});x.controller.invalidate()
    expect(x.check.mock.calls[0][1].aborted).toBe(true)
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run
    expect(x.controller.snapshot()).toEqual({phase:'idle',busy:false})
  })
  it('drops a changed input even when completion precedes a renderer invalidation effect',async()=>{
    const x=setup(),run=x.controller.start('fixture-target');x.change({proxyConfig:'http://other.example.invalid:8080'})
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run;expect(x.controller.snapshot()).toEqual({phase:'idle',busy:false})
  })
  it('clears previously completed evidence on input/target change',async()=>{
    const x=setup(),run=x.controller.start('fixture-target');x.pending.resolve(result(x.check.mock.calls[0][0]));await run
    x.controller.invalidate();expect(x.controller.snapshot()).toEqual({phase:'idle',busy:false})
  })
  it('does not attribute an old timeout to a replacement scope before renderer invalidation',async()=>{
    vi.useFakeTimers();const x=setup({timeoutMs:100}),run=x.controller.start('fixture-target')
    x.change({scope:{...scope,revision:scope.revision+1}});await vi.advanceTimersByTimeAsync(100)
    expect(x.controller.snapshot()).toMatchObject({phase:'invalidated',reason:'CONTEXT_CHANGED'})
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run
    expect(x.controller.snapshot()).toEqual({phase:'idle',busy:false})
  })
  it('disposes, aborts and never calls a listener or captured action after unmount',async()=>{
    const x=setup(),listener=vi.fn(),unsubscribe=x.controller.subscribe(listener),run=x.controller.start('fixture-target')
    unsubscribe();x.controller.dispose();listener.mockClear();expect(x.check.mock.calls[0][1].aborted).toBe(true)
    x.pending.resolve(result(x.check.mock.calls[0][0]));await run;await x.controller.start('fixture-target')
    expect(listener).not.toHaveBeenCalled();expect(x.check).toHaveBeenCalledTimes(1)
  })
  it('retains the operation lock across adapter replacement, even before invalidation effects',async()=>{
    const p=deferred(),old={targets,check:vi.fn(()=>p.promise)},replacement={targets,check:vi.fn(async r=>result(r))}
    let port=old
    const controller=createProxyCheckController(()=>port,()=>({scope,proxyConfig:'http://proxy.example.invalid:8080',enabled:true}),{id:()=> 'fixture-request'})
    const run=controller.start('fixture-target');port=replacement
    await controller.start('fixture-target');expect(replacement.check).not.toHaveBeenCalled()
    p.resolve(result(old.check.mock.calls[0][0]));await run
    expect(controller.snapshot()).toEqual({phase:'idle',busy:false})
    await controller.start('fixture-target');expect(replacement.check).toHaveBeenCalledTimes(1)
  })
})
