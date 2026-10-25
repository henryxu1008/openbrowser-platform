import { captureBridge } from '../../auth/bridge'
import type { BrowserCore, BrowserProfileInput } from '../browser/types'

export type Store = { id: string; name: string; platform: string; region: string; recordStatus: string; serviceStatus?: string; connectionStatus?: string }
export type StoreContext = { context: { user: { id: string }; effectiveOrganizationId: string | null; selectionRequired: boolean; organizations: { id: string; name: string; permissions: string[] }[] }; revision: number }
export type Catalog = { items: Store[]; pagination: { page: number; pageSize: number; total: number } }
export type Binding = { bindingId: string; profileId: string; bindingVersion: string }
export type Operation = { operationId: string; action: string; phase: string; outcome: string; reasonCode: string; reportState: string }
export type Environment = { binding: Binding | null; name: string; coreId: string; proxySummary: string; running: boolean; runtimeKnown: boolean; pending: Operation | null; lastOperation: Operation | null; reports: { kind: string; outcome: string; confirmed: boolean }[]; observedAt: string }
export type AuditItem = { eventId: string; operationId: string; actorSubject: string; memberRef: string; action: string; serverTime: string; clientObservedAt?: string; outcome: string; reasonCode: string; profileRef: string; bindingRef: string; factSource: string }
export type AuditPage = { items: AuditItem[]; nextCursor: string | null; coverage: string[]; canFilterActor: boolean }
export type AuditQuery = { action?: string; outcome?: string; actor?: string; from?: string; to?: string; cursor?: string }

// Capture the original auth epoch once for a page operation. Native also pins
// generation and organization revision; no token/URL/header transport is exposed.
export function storeAPI() {
  const native = captureBridge()
  const json = async <T>(promise: Promise<string>): Promise<T> => JSON.parse(await promise) as T
  return {
    context: () => json<StoreContext>(native.StoreWorkbenchContext()),
    select: (org: string, revision: number) => json<StoreContext>(native.StoreWorkbenchSelectOrganization(org, revision)),
    list: (q: string, platform: string, page: number, revision: number) => json<Catalog>(native.StoreWorkbenchList(q, platform, page, revision)),
    store: (id: string, revision: number) => json<Store>(native.StoreWorkbenchGet(id, revision)),
    environment: (id: string, revision: number) => json<Environment>(native.StoreEnvironmentGet(id, revision)),
    options: (id: string, revision: number) => json<{input: BrowserProfileInput; cores: BrowserCore[]; binding: Binding | null}>(native.StoreEnvironmentOptions(id, revision)),
    save: (id: string, version: string, input: BrowserProfileInput, revision: number) => json<Environment>(native.StoreEnvironmentSave(id, version, JSON.stringify(input), revision)),
    open: (id: string, version: string, revision: number) => json<Environment>(native.StoreEnvironmentOpen(id, version, revision)),
    close: (id: string, revision: number) => json<{operationId:string;outcome:string;observedAt:string}>(native.StoreEnvironmentClose(id, revision)),
    receipt: (id: string, key: string, revision: number) => json<unknown>(native.StoreOperationGet(id, key, revision)),
    audit: (id: string, query: AuditQuery, revision: number) => json<AuditPage>(native.StoreAuditList(id, JSON.stringify(query), revision)),
  }
}
export function storeError(error: unknown) {
  const code = String(error)
  if (code.includes('store_service_configuration_required')) return '店铺服务尚未配置，请联系安装包维护者。'
  if (code.includes('PERMISSION') || code.includes('NOT_FOUND')) return '当前身份无权访问该店铺或执行此操作。'
  if (code.includes('BINDING_VERSION_CONFLICT')) return '环境已发生变化，请返回店铺核对后再操作。'
  if (code.includes('store_kernel_required')) return '尚未安装可用内核，请先使用内核管理完成安装。'
  if (code.includes('unknown') || code.includes('context_changed')) return `结果待核对，已停止操作。请查看原操作记录，不要重复打开。${storeOperationKey(error) ? ` 操作编号：${storeOperationKey(error)}` : ''}`
  return `店铺服务不可用，操作已停止。${code}`
}
export function storeOperationKey(error:unknown){return String(error).match(/operationId=([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/)?.[1]||''}
