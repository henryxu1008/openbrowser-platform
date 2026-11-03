import { expect, it, vi } from 'vitest'
import { buildStoreProxyDraft } from '../src/modules/stores/proxyDraft'
import { INITIAL_DIRECT_IMPORT_FORM } from '../src/modules/browser/components/ProxyImportModal.types'

const form = (values = {}) => ({ ...INITIAL_DIRECT_IMPORT_FORM, server: 'proxy.example.invalid', port: '8080', ...values })
it.each(['http', 'https', 'socks5'])('encodes credentials exactly once for %s without changing the password', protocol => {
  const username = '合成 user:@/%?#', password = ' p:@/%?#合成 '
  const result = buildStoreProxyDraft(form({ protocol, username, password }))
  expect(result.error).toBeUndefined()
  const url = new URL(result.proxyConfig)
  expect(url.protocol).toBe(protocol + ':')
  expect(decodeURIComponent(url.username)).toBe(username)
  expect(decodeURIComponent(url.password)).toBe(password)
  expect(url.hostname).toBe('proxy.example.invalid')
})
it.each(['127.0.0.1', '::1', '[2001:db8::1]', '代理.example.invalid'])('supports the existing builder host formats: %s', server => {
  const result = buildStoreProxyDraft(form({ server }))
  expect(result.error).toBeUndefined()
  expect(new URL(result.proxyConfig).port).toBe('8080')
})
it.each([
  { server: '' }, { server: ' ' }, { port: '' }, { port: '0' }, { port: '65536' }, { port: '1.2' }, { port: '1e3' },
  { server: 'http://proxy.example.invalid' }, { server: 'user:secret@proxy.example.invalid' },
  { server: 'proxy.example.invalid/path' }, { server: 'proxy.example.invalid?secret' }, { server: 'proxy.example.invalid#secret' },
  { server: 'proxy.example.invalid\\secret' }, { server: 'proxy%2eexample.invalid' }, { server: 'bad host' },
  { server: '[broken' }, { protocol: 'direct' }, { username: '', password: 'fixture-secret' },
  { username: ' space ', password: 'fixture-secret' }, { username: 'fixture', password: 'fixture-secret\r\n' },
  { protocol: 'socks5', username: 'fixture', password: '' },
  { protocol: 'socks5', username: '中'.repeat(86), password: 'fixture-secret' },
  { protocol: 'socks5', username: 'fixture', password: '中'.repeat(86) },
  { username: 'fixture', password: 'fixture-secret'.repeat(1000) },
  { username: '\ud800', password: 'fixture-secret' },
])('rejects invalid parameters with a credential-free error: %j', values => {
  const result = buildStoreProxyDraft(form(values))
  expect(result.proxyConfig).toBeUndefined()
  expect(result.error).toBeTruthy()
  expect(result.error).not.toContain('fixture-secret')
  expect(result.error).not.toContain('user:secret@')
})
it('constructs drafts without persistence, storage, logging or network calls', () => {
  const fetch = vi.fn(() => { throw new Error('network forbidden') }), log = vi.spyOn(console, 'log').mockImplementation(() => {})
  vi.stubGlobal('fetch', fetch)
  try {
    expect(buildStoreProxyDraft(form()).proxyConfig).toBe('http://proxy.example.invalid:8080')
    expect(fetch).not.toHaveBeenCalled(); expect(log).not.toHaveBeenCalled()
  } finally { log.mockRestore(); vi.unstubAllGlobals() }
})
