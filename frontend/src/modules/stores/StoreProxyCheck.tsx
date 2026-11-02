import { useEffect, useRef, useState } from 'react'
import { Button, FormItem, Select } from '../../shared/components'
import { createProxyCheckController, scopeKey, type CheckScope, type CheckView, type ProxyCheckPort } from './proxyCheck'

const reasonText = {
  NONE: '', INVALID_PROXY: '请填写有效的自有代理地址；直连不执行代理检查。',
  UNSUPPORTED_PROTOCOL: '此检查暂不支持该协议，原配置不变。',
  TCP_UNREACHABLE: '代理服务器 TCP 不可达，请核对地址、端口和服务状态。',
  TARGET_UNREACHABLE: '未能经代理访问所选目标，请核对代理与目标服务。',
  AUTH_REJECTED: '代理明确拒绝了认证，请核对代理凭据。',
  TIMEOUT: '检查超时，结果未知。', CANCELED: '已取消检查，结果未知。',
  UNAVAILABLE: '暂时无法检查，请稍后重试或联系维护者。',
  PERMISSION_DENIED: '当前身份无权检查此店铺网络，请联系企业管理员。',
  CONTEXT_CHANGED: '店铺、企业、身份或配置已变化，原检查结果已失效。',
}
const evidenceText = { passed: '通过', failed: '未通过', not_checked: '未检查' }

// An optional editor extension only. Production Store adapter intentionally does
// not provide this port until the native activation contract has been reviewed.
export function StoreProxyCheck({ port, scope, proxyConfig, disabled = false }: {
  port: ProxyCheckPort; scope: CheckScope; proxyConfig: string; disabled?: boolean
}) {
  const [target, setTarget] = useState('')
  const current = useRef({ scope, proxyConfig, enabled: !disabled })
  current.current = { scope, proxyConfig, enabled: !disabled }
  const currentPort = useRef(port)
  currentPort.current = port
  const active = useRef<ReturnType<typeof createProxyCheckController> | null>(null)
  const owner = scopeKey(scope)
  const [received, setReceived] = useState<{ view: CheckView; owner: string; input: string; port: ProxyCheckPort }>({
    view: { phase: 'idle', busy: false }, owner, input: proxyConfig, port,
  })
  useEffect(() => {
    // The operation owner lives for this component, across adapter replacement.
    // Aborting an old adapter never proves its sockets/worker have stopped.
    const controller = createProxyCheckController(() => currentPort.current, () => current.current)
    active.current = controller
    const unsubscribe = controller.subscribe(view => setReceived({ view,
      owner: scopeKey(current.current.scope), input: current.current.proxyConfig, port: currentPort.current }))
    return () => { unsubscribe(); controller.dispose(); if (active.current === controller) active.current = null }
  }, [])
  useEffect(() => { active.current?.invalidate() }, [port, owner, proxyConfig, disabled, target])
  // Hide old evidence in the same render that changes input/scope, before effects.
  const view = received.owner === owner && received.input === proxyConfig && received.port === port ? received.view : { phase: 'idle', busy: received.view.busy } as CheckView
  const result = view.result
  return <div className="store-proxy-check mt-4 space-y-3 border-t border-[var(--color-border-default)] pt-4">
    <FormItem label="检查目标">
      <Select aria-label="检查目标" className="min-h-[44px]" value={target} disabled={disabled || view.busy}
        onChange={e => setTarget(e.target.value)} options={[
          { value: '', label: '请选择一个检查目标' },
          ...port.targets.map(t => ({ value: t.id, label: t.label })),
        ]} />
    </FormItem>
    {port.targets.find(t => t.id === target) && <p className="break-all text-xs text-[var(--color-text-secondary)]">目标地址：{port.targets.find(t => t.id === target)?.url}</p>}
    <p className="text-xs text-[var(--color-text-secondary)]">仅检查当前代理与所选目标，不保存配置、不打开环境。结果不证明地区正确或商家账号可登录。</p>
    <div className="flex flex-wrap gap-2">
      <Button type="button" variant="secondary" className="border-solid min-h-[44px]" disabled={disabled || view.busy || !target || !proxyConfig.trim()}
        onClick={() => {
          if (currentPort.current === port && scopeKey(current.current.scope) === owner && current.current.proxyConfig === proxyConfig) void active.current?.start(target)
        }}>检查此代理</Button>
      {view.busy && <Button type="button" variant="secondary" className="border-solid min-h-[44px]" disabled={view.phase !== 'checking'}
        onClick={() => { if (currentPort.current === port) active.current?.cancel() }}>取消检查</Button>}
    </div>
    <div role="status" aria-live="polite" aria-atomic="true" className="space-y-2 text-sm text-[var(--color-text-secondary)]">
      {view.phase === 'checking' && <p>正在检查所选目标…</p>}
      {view.reason && <p>{reasonText[view.reason]}{view.busy ? ' 等待检查停止后可重试。' : ''}</p>}
      {result && <>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
          <dt>代理格式</dt><dd>{evidenceText[result.syntax]}</dd>
          <dt>代理服务器 TCP</dt><dd>{evidenceText[result.tcp]}</dd>
          <dt>经代理访问所选目标</dt><dd>{evidenceText[result.target]}</dd>
        </dl>
        {result.reason !== 'NONE' && <p>{reasonText[result.reason]}</p>}
        <p className="text-xs text-[var(--color-text-secondary)]">仅代表本次所选目标的检查结果；保存与启动仍需分别操作。</p>
      </>}
    </div>
  </div>
}
