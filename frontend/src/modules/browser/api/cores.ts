import type { BrowserCore } from '../types'
import { getBindings } from './runtime'

export async function fetchBrowserCores(): Promise<BrowserCore[]> {
  const bindings = await getBindings()
  if (!bindings?.BrowserCoreList) throw new Error('kernel_bridge_unavailable')
  return (await bindings.BrowserCoreList()) || []
}

// Existing automation output navigation also uses this general file action.
export async function openCorePath(corePath: string): Promise<boolean> {
  const bindings = await getBindings()
  if (!bindings?.OpenCorePath) return false
  await bindings.OpenCorePath(corePath)
  return true
}
