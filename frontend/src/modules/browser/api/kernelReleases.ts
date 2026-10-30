import { loadBusinessBridge } from '../../../auth/loadBridge'

export interface KernelRelease {
  releaseId: string
  version: string
  os: string
  arch: string
  minClientVersion: string
  maxClientVersionExclusive: string
  recommended: boolean
  installed: boolean
}

// Capture the session before the dynamic import. No preview/mock fallback,
// artifact metadata or paths cross this adapter.
export async function fetchKernelReleases(): Promise<KernelRelease[]> {
  const bridge = await loadBusinessBridge()
  if (typeof bridge.BrowserKernelCatalog !== 'function') throw new Error('kernel_bridge_unavailable')
  const releases: unknown = JSON.parse(await bridge.BrowserKernelCatalog())
  if (!Array.isArray(releases)) throw new Error('kernel_catalog_unavailable')
  return releases
}

export async function prepareKernelRelease(releaseId: string): Promise<void> {
  const bridge = await loadBusinessBridge()
  if (typeof bridge.BrowserKernelPrepare !== 'function') throw new Error('kernel_bridge_unavailable')
  await bridge.BrowserKernelPrepare(releaseId)
}

export function kernelError(error: unknown): string {
  const code = String(error)
  if (code.includes('configuration')) return '内核版本服务尚未配置，请联系平台管理员。'
  if (code.includes('not_found')) return '此版本已撤回或不适用于当前客户端，请刷新版本列表。'
  if (code.includes('identity_changed')) return '此版本的发行信息不一致，请联系平台管理员。'
  if (code.includes('authentication') || code.includes('unauthenticated')) return '登录状态已变化，请重新登录后选择。'
  if (code.includes('busy')) return '此版本仍在处理前一会话的请求，请稍后重试。'
  if (code.includes('integrity') || code.includes('archive')) return '内核制品校验失败，请联系平台管理员。'
  return '无法取得或准备此内核版本，请稍后重试。'
}
