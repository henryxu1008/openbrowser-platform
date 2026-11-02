// Draft offline port. No production native binding is connected to this module.
export type CheckScope = {
  identity: string; generation: number; organization: string; revision: number
  store: string; bindingVersion: string; connector: 'xray' | 'mihomo'
}
export type CheckTarget = { id: string; label: string; url: string }
export type Evidence = 'passed' | 'failed' | 'not_checked'
export type CheckReason = 'NONE' | 'INVALID_PROXY' | 'UNSUPPORTED_PROTOCOL' | 'TCP_UNREACHABLE' |
  'TARGET_UNREACHABLE' | 'AUTH_REJECTED' | 'TIMEOUT' | 'CANCELED' | 'UNAVAILABLE' |
  'PERMISSION_DENIED' | 'CONTEXT_CHANGED'
export type CheckResult = {
  schema: 'store-proxy-check-v1'; requestId: string; targetId: string
  syntax: Evidence; tcp: Evidence; target: Evidence; reason: CheckReason
}
export type CheckRequest = { requestId: string; scope: CheckScope; proxyConfig: string; targetId: string }
export type ProxyCheckPort = {
  targets: readonly CheckTarget[]
  // Native must derive authority independently, enforce binding CAS, whitelist
  // results, honor cancellation and settle only after owned resources stop.
  check: (request: CheckRequest, signal: AbortSignal) => Promise<unknown>
}
export type CheckView = {
  phase: 'idle' | 'checking' | 'canceled' | 'timed_out' | 'invalidated' | 'result' | 'failed'
  busy: boolean; reason?: CheckReason; result?: CheckResult
}
type Current = { scope: CheckScope; proxyConfig: string; enabled: boolean }

export function scopeKey(scope: CheckScope) {
  return JSON.stringify([scope.identity, scope.generation, scope.organization, scope.revision,
    scope.store, scope.bindingVersion, scope.connector])
}

// Conservative syntax support for this check, not the browser's protocol matrix.
export function inspectProxyInput(raw: string): CheckReason | null {
  const value = raw.trim()
  if (!value || new TextEncoder().encode(raw).byteLength > 8192 || /[\s\x00-\x20\x7f]/.test(value) || value.toLowerCase() === 'direct://') return 'INVALID_PROXY'
  if (!/^(https?|socks5):\/\//i.test(value)) return 'UNSUPPORTED_PROTOCOL'
  try {
    const url = new URL(value)
    if (/^[a-z0-9]+:\/\/\//i.test(value) || !url.hostname || url.search || url.hash || (url.pathname && url.pathname !== '/') ||
      (url.port && (Number(url.port) < 1 || Number(url.port) > 65535)) ||
      (url.protocol === 'socks5:' && !url.port)) return 'INVALID_PROXY'
    return null
  } catch { return 'INVALID_PROXY' }
}

const evidence = new Set(['passed', 'failed', 'not_checked'])
const reasons = new Set(['NONE', 'INVALID_PROXY', 'UNSUPPORTED_PROTOCOL', 'TCP_UNREACHABLE',
  'TARGET_UNREACHABLE', 'AUTH_REJECTED', 'TIMEOUT', 'CANCELED', 'UNAVAILABLE', 'PERMISSION_DENIED', 'CONTEXT_CHANGED'])

// Never store/spread raw diagnostics or interpolate an exception into UI text.
export function safeCheckResult(raw: unknown, request: Pick<CheckRequest, 'requestId' | 'targetId'>): CheckResult | null {
  if (!raw || typeof raw !== 'object') return null
  const r = raw as Record<string, unknown>
  if (r.schema !== 'store-proxy-check-v1' || r.requestId !== request.requestId || r.targetId !== request.targetId ||
    typeof r.syntax !== 'string' || !evidence.has(r.syntax) || typeof r.tcp !== 'string' || !evidence.has(r.tcp) ||
    typeof r.target !== 'string' || !evidence.has(r.target) || typeof r.reason !== 'string' || !reasons.has(r.reason)) return null
  if ((r.tcp !== 'not_checked' && r.syntax !== 'passed') ||
    (r.target !== 'not_checked' && r.tcp !== 'passed') ||
    (r.reason === 'NONE' && r.target !== 'passed') ||
    (r.target === 'passed' && r.reason !== 'NONE') ||
    (['INVALID_PROXY', 'UNSUPPORTED_PROTOCOL'].includes(r.reason) && (r.syntax !== 'failed' || r.tcp !== 'not_checked')) ||
    (r.reason === 'TCP_UNREACHABLE' && (r.syntax !== 'passed' || r.tcp !== 'failed')) ||
    (['TARGET_UNREACHABLE', 'AUTH_REJECTED'].includes(r.reason) && r.target !== 'failed') ||
    (['PERMISSION_DENIED', 'CONTEXT_CHANGED'].includes(r.reason) && r.syntax !== 'not_checked')) return null
  return { schema: 'store-proxy-check-v1', requestId: request.requestId, targetId: request.targetId,
    syntax: r.syntax as Evidence, tcp: r.tcp as Evidence, target: r.target as Evidence, reason: r.reason as CheckReason }
}

export function createProxyCheckController(source: ProxyCheckPort | (() => ProxyCheckPort), current: () => Current,
  options: { timeoutMs?: number; id?: () => string } = {}) {
  const readPort = () => typeof source === 'function' ? source() : source
  let view: CheckView = { phase: 'idle', busy: false }
  let disposed = false
  let active: { abort: AbortController; key: string; input: string; invalidated: boolean; port: ProxyCheckPort } | null = null
  const listeners = new Set<(view: CheckView) => void>()
  const emit = (next: CheckView) => { view = next; if (!disposed) listeners.forEach(fn => fn(next)) }
  const matches = (operation: NonNullable<typeof active>) => {
    const c = current()
    return c.enabled && readPort() === operation.port && scopeKey(c.scope) === operation.key && c.proxyConfig === operation.input
  }
  return {
    snapshot: () => view,
    subscribe(fn: (view: CheckView) => void) { listeners.add(fn); fn(view); return () => { listeners.delete(fn) } },
    invalidate() {
      if (disposed) return
      if (active) {
        active.invalidated = true; active.abort.abort()
        emit({ phase: 'invalidated', busy: true, reason: 'CONTEXT_CHANGED' })
      } else emit({ phase: 'idle', busy: false })
    },
    cancel() {
      if (!active || disposed || view.phase !== 'checking') return
      active.abort.abort()
      emit({ phase: 'canceled', busy: true, reason: 'CANCELED' })
    },
    dispose() { disposed = true; active?.abort.abort(); listeners.clear() },
    async start(targetId: string) {
      if (disposed || active) return
      const c = current()
      if (!c.enabled) return
      const port = readPort()
      const invalid = inspectProxyInput(c.proxyConfig)
      if (invalid) { emit({ phase: 'failed', busy: false, reason: invalid }); return }
      if (!targetId || !port.targets.some(t => t.id === targetId)) {
        emit({ phase: 'failed', busy: false, reason: 'UNAVAILABLE' }); return
      }
      const operation = { abort: new AbortController(), key: scopeKey(c.scope), input: c.proxyConfig, invalidated: false, port }
      active = operation
      const request = { requestId: options.id?.() || crypto.randomUUID(), scope: { ...c.scope }, proxyConfig: c.proxyConfig.trim(), targetId }
      emit({ phase: 'checking', busy: true })
      const timeout = setTimeout(() => {
        if (disposed || active !== operation || operation.abort.signal.aborted) return
        if (!matches(operation)) {
          operation.invalidated = true; operation.abort.abort()
          emit({ phase: 'invalidated', busy: true, reason: 'CONTEXT_CHANGED' }); return
        }
        operation.abort.abort()
        emit({ phase: 'timed_out', busy: true, reason: 'TIMEOUT' })
      }, Math.min(12000, Math.max(1, options.timeoutMs ?? 12000)))
      try {
        const raw = await port.check(request, operation.abort.signal)
        if (disposed) return
        if (!matches(operation)) operation.invalidated = true
        if (operation.invalidated) emit({ phase: 'idle', busy: false })
        else if (operation.abort.signal.aborted) emit({ ...view, busy: false, result: undefined })
        else {
          const result = safeCheckResult(raw, request)
          emit(result ? { phase: 'result', busy: false, result } : { phase: 'failed', busy: false, reason: 'UNAVAILABLE' })
        }
      } catch {
        if (!disposed) {
          if (!matches(operation) || operation.invalidated) emit({ phase: 'idle', busy: false })
          else if (operation.abort.signal.aborted) emit({ ...view, busy: false, result: undefined })
          else emit({ phase: 'failed', busy: false, reason: 'UNAVAILABLE' })
        }
      } finally { clearTimeout(timeout); if (active === operation) active = null }
    },
  }
}
