import { buildDirectImportCandidate } from '../browser/components/ProxyImportModal.helpers'
import type { DirectImportForm } from '../browser/components/ProxyImportModal.types'
import { inspectProxyInput } from './proxyCheck'

const safeErrors = new Set([
  '请输入代理地址', '代理地址只需要填写域名或 IP，不需要协议头',
  '请输入代理端口', '代理端口必须为整数', '代理端口必须在 1-65535 之间',
  '填写密码时请同时填写账号', '请输入有效的代理地址',
])

// Pure draft construction. No pool, persistence, native call or network access.
export function buildStoreProxyDraft(form: DirectImportForm): { proxyConfig: string; error?: never } | { error: string; proxyConfig?: never } {
  if (!['http', 'https', 'socks5'].includes(form.protocol)) return { error: '请选择 HTTP、HTTPS 或 SOCKS5 协议。' }
  // The host field is an endpoint, never a URI/userinfo/path. Reject delimiters
  // before the shared builder can interpret them as a different authority.
  if (/[\s/@\\?#%\x00-\x1f\x7f]/u.test(form.server.trim())) return { error: '代理地址只填写域名或 IP，不包含协议、账号或路径。' }
  if (form.username !== form.username.trim()) return { error: '账号首尾请勿留空格；密码按原样保留。' }
  if (/[\x00-\x1f\x7f]/.test(form.username + form.password)) return { error: '账号和密码不能包含控制字符。' }
  if (form.protocol === 'socks5' && (form.username || form.password) &&
      (!form.username || !form.password || new TextEncoder().encode(form.username).length > 255 || new TextEncoder().encode(form.password).length > 255)) {
    return { error: 'SOCKS5 认证需要同时填写账号和密码，每项最多 255 字节。' }
  }
  try {
    const { proxyConfig } = buildDirectImportCandidate(form)
    if (inspectProxyInput(proxyConfig)) return { error: '代理参数无效或过长，请检查后重试。' }
    return { proxyConfig }
  } catch (error) {
    // The shared builder currently uses fixed validation messages. Never echo
    // unrecognized exceptions, which could include credential-bearing input.
    return { error: error instanceof Error && safeErrors.has(error.message) ? error.message : '代理参数无效，请检查地址、端口和账号。' }
  }
}
