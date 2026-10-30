import { useEffect, useRef, useState } from 'react'
import { Button, Select } from '../../../shared/components'
import { authEpoch, authSnapshot, signedIn, subscribeAuth } from '../../../auth/session'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { fetchKernelReleases, kernelError, prepareKernelRelease, type KernelRelease } from '../api/kernelReleases'

interface Props {
  value: string
  scopeKey: string
  disabled?: boolean
  onChange: (releaseId: string) => void
  onBusyChange?: (busy: boolean) => void
}
const phases: Record<string, string> = { downloading: '正在准备', verifying: '正在校验', extracting: '正在解压', ready: '已就绪' }

// Preparing a choice caches a public artifact; the parent owns Save/Cancel.
export function KernelVersionPicker({ value, scopeKey, disabled, onChange, onBusyChange }: Props) {
  const [releases, setReleases] = useState<KernelRelease[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [pending, setPending] = useState('')
  const [error, setError] = useState('')
  const [progress, setProgress] = useState('')
  const serial = useRef(0)
  const working = useRef(false)
  const callbacks = useRef({ onChange, onBusyChange })
  callbacks.current = { onChange, onBusyChange }
  const context = useRef({ scopeKey, value })
  context.current = { scopeKey, value }

  const ticket = () => {
    const request = ++serial.current, generation = authSnapshot().generation, epoch = authEpoch()
    return { generation, live: () => request === serial.current && scopeKey === context.current.scopeKey && value === context.current.value && generation === authSnapshot().generation && epoch === authEpoch() && signedIn() }
  }
  const refresh = async () => {
    if (working.current) return
    const request = ticket()
    setLoading(true); setError('')
    try {
      const list = await fetchKernelReleases()
      if (request.live()) setReleases(list)
    } catch (e) { if (request.live()) setError(kernelError(e)) }
    finally { if (request.live()) setLoading(false) }
  }

  useEffect(() => {
    working.current = false
    setBusy(false); setPending(''); setProgress('')
    callbacks.current.onBusyChange?.(false)
    void refresh()
    const unsubscribe = subscribeAuth(() => {
      serial.current++
      working.current = false
      setReleases([]); setBusy(false); setPending(''); setLoading(false)
      setError('登录状态已变化，请重新登录后选择。')
      callbacks.current.onBusyChange?.(false)
    })
    return () => { serial.current++; unsubscribe(); callbacks.current.onBusyChange?.(false) }
  }, [scopeKey, value])

  const choose = async (releaseId: string) => {
    if (!releaseId || working.current || disabled || loading) return
    const release = releases.find(item => item.releaseId === releaseId)
    if (!release) return
    const request = ticket()
    if (!request.live()) return
    working.current = true
    setBusy(true); setPending(releaseId); setError(''); setProgress('正在准备所选版本…')
    callbacks.current.onBusyChange?.(true)
    // The per-listener disposer never removes another page's listener.
    let off = () => {}
    try {
      off = EventsOn('browser.kernel.progress', (event: { releaseId: string; generation: number; phase: string; percent: number }) => {
        if (request.live() && event?.releaseId === releaseId && event.generation === request.generation && phases[event.phase] && Number.isFinite(event.percent)) {
          setProgress(`${phases[event.phase]} ${Math.max(0, Math.min(100, event.percent))}%`)
        }
      })
      await prepareKernelRelease(releaseId)
      if (!request.live()) return
      setReleases(list => list.map(item => item.releaseId === releaseId ? { ...item, installed: true } : item))
      setProgress('所选版本已就绪。')
      callbacks.current.onChange(releaseId)
    } catch (e) { if (request.live()) setError(kernelError(e)) }
    finally {
      off()
      if (request.live()) { working.current = false; setBusy(false); setPending(''); callbacks.current.onBusyChange?.(false) }
    }
  }
  const unavailable = !!value && !releases.some(item => item.releaseId === value)
  const needsPreparation = releases.some(item => item.releaseId === value && !item.installed)
  return <div className="space-y-2">
    <div className="flex gap-2">
      <Select aria-label="内核版本" value={pending || value} disabled={disabled || loading || busy} onChange={event => void choose(event.target.value)} options={[
        { value: '', label: loading ? '正在获取可用版本…' : '请选择平台发行版本' },
        ...(unavailable ? [{ value, label: '当前绑定版本（不在可选目录中）' }] : []),
        ...releases.map(item => ({ value: item.releaseId, label: `${item.version}${item.recommended ? ' · 推荐' : ''}${item.installed ? ' · 已就绪' : ''}` })),
      ]} />
      <Button variant="secondary" disabled={disabled || loading || busy} onClick={() => void refresh()}>刷新版本</Button>
    </div>
    {needsPreparation && <Button variant="secondary" disabled={disabled || loading || busy} onClick={() => void choose(value)}>准备此版本</Button>}
    {!loading && !error && !releases.length && <p className="text-sm text-[var(--color-text-muted)]">暂无适用于此客户端的发行版本。</p>}
    {unavailable && <p className="text-sm text-[var(--color-text-muted)]">现有绑定保持原版本；选择新版本后，保存配置才会更改绑定。</p>}
    {progress && <p role="status" className="text-sm text-[var(--color-text-secondary)]">{progress}</p>}
    {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
  </div>
}
