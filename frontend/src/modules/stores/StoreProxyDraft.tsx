import { useEffect, useId, useRef, useState } from 'react'
import { subscribeAuth } from '../../auth/session'
import { Button, FormItem, Input, Modal, Select } from '../../shared/components'
import { DIRECT_PROXY_PROTOCOL_OPTIONS, INITIAL_DIRECT_IMPORT_FORM, type DirectImportForm } from '../browser/components/ProxyImportModal.types'
import { buildStoreProxyDraft } from './proxyDraft'

type Draft = { token: object; context: object; source: string; form: DirectImportForm; error: string }
type Props = { context: object; proxyConfig: string; disabled: boolean; onApply: (proxyConfig: string) => boolean }

export function StoreProxyDraft({ context, proxyConfig, disabled, onApply }: Props) {
  const [draft, setDraft] = useState<Draft | null>(null)
  const [applied, setApplied] = useState<{ context: object; proxyConfig: string } | null>(null)
  const token = useRef<object | null>(null), active = useRef(false)
  const current = useRef({ context, proxyConfig, disabled })
  current.current = { context, proxyConfig, disabled }
  const errorId = useId(), hintId = useId(), formId = useId()
  const clear = () => { token.current = null; setDraft(null) }
  useEffect(() => {
    active.current = true
    const unsubscribe = subscribeAuth(() => { clear(); setApplied(null) })
    return () => { active.current = false; token.current = null; unsubscribe() }
  }, [])
  useEffect(() => { clear() }, [context, disabled, proxyConfig])
  useEffect(() => { setApplied(null) }, [context, disabled])
  const live = (value: Draft) => active.current && token.current === value.token &&
    current.current.context === value.context && current.current.proxyConfig === value.source && !current.current.disabled
  const close = () => { if (draft && token.current === draft.token) clear() }
  const change = (field: keyof DirectImportForm, value: string) => {
    if (!draft || !live(draft)) return
    setDraft(previous => previous?.token === draft.token ? { ...previous, form: { ...previous.form, [field]: value }, error: '' } : previous)
  }
  const apply = () => {
    if (!draft || !live(draft)) return
    const result = buildStoreProxyDraft(draft.form)
    if (result.error !== undefined) { setDraft({ ...draft, error: result.error }); return }
    // Consume before calling the parent so duplicate/captured callbacks cannot
    // apply twice. The parent independently keeps its existing editor guards.
    clear()
    if (onApply(result.proxyConfig)) setApplied({ context, proxyConfig: result.proxyConfig })
  }
  const visible = !!draft && live(draft)
  return <div className="mt-3 space-y-2">
    <Button type="button" variant="secondary" size="sm" disabled={disabled} onClick={() => {
      if (!active.current || current.current.disabled || token.current || current.current.context !== context || current.current.proxyConfig !== proxyConfig) return
      const next = { token: {}, context, source: proxyConfig, form: { ...INITIAL_DIRECT_IMPORT_FORM }, error: '' }
      token.current = next.token; setDraft(next); setApplied(null)
    }}>填写代理参数</Button>
    {applied?.context === context && applied.proxyConfig === proxyConfig && <p role="status" className="text-xs text-[var(--color-text-muted)]">代理参数已应用到当前草稿，尚未保存或检测。</p>}
    {visible && <Modal className="store-workbench-modal" open title="填写自有代理参数" width="600px" onClose={close} footer={<>
      <Button type="button" variant="secondary" onClick={close}>取消</Button>
      <Button type="submit" form={formId}>应用到草稿</Button>
    </>}>
      <form id={formId} aria-label="自有代理参数" aria-describedby={`${hintId}${draft.error ? ` ${errorId}` : ''}`} onSubmit={event => { event.preventDefault(); apply() }}>
        <p id={hintId} className="mb-4 text-sm text-[var(--color-text-secondary)]">只填写当前店铺的自有代理；应用后仍需保存配置。此处不会检查连接。</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <FormItem label="代理协议" required className="sm:col-span-2"><Select aria-label="代理协议" options={[...DIRECT_PROXY_PROTOCOL_OPTIONS]} value={draft.form.protocol} onChange={event => change('protocol', event.target.value)} /></FormItem>
          <FormItem label="代理地址" required><Input aria-label="代理地址" autoComplete="off" spellCheck={false} value={draft.form.server} onChange={event => change('server', event.target.value)} placeholder="域名或 IP" aria-describedby={draft.error ? errorId : undefined} /></FormItem>
          <FormItem label="代理端口" required><Input aria-label="代理端口" type="text" inputMode="numeric" autoComplete="off" value={draft.form.port} onChange={event => change('port', event.target.value)} placeholder="1–65535" /></FormItem>
          <FormItem label="账号（可选）"><Input aria-label="代理账号" autoComplete="off" spellCheck={false} value={draft.form.username} onChange={event => change('username', event.target.value)} /></FormItem>
          <FormItem label="密码（可选）"><Input aria-label="代理密码" type="password" autoComplete="new-password" value={draft.form.password} onChange={event => change('password', event.target.value)} /></FormItem>
        </div>
        {draft.error && <p id={errorId} role="alert" className="mt-3 text-sm text-[var(--color-error)]">{draft.error}</p>}
      </form>
    </Modal>}
  </div>
}
