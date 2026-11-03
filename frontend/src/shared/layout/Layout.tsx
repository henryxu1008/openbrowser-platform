import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Bell, HelpCircle, Menu, Search } from 'lucide-react'
import { Sidebar } from './Sidebar'
import { ShellSlotsProvider, useShellSlots } from './ShellSlots'
import { useNotificationStore } from '../../store/notificationStore'
import './shell.css'

export function Layout({ children }: { children: ReactNode }) {
  const slots = useShellSlots()
  return slots ? <Shell>{children}</Shell> : <ShellSlotsProvider><Shell>{children}</Shell></ShellSlotsProvider>
}
function Shell({ children }: { children: ReactNode }) {
  const location = useLocation(), navigate = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)
  const closeMobile = useCallback(() => setMobileOpen(false), [])
  const workspaceRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const query = new URLSearchParams(location.search).get('q') || ''
  const [draft, setDraft] = useState(query)
  const canSearch = location.pathname === '/stores'
  const unread = useNotificationStore(state => state.notifications.filter(item => !item.read).length)
  useEffect(() => { setDraft(query) }, [query])
  useEffect(() => { setMobileOpen(false) }, [location.pathname])
  useEffect(() => { if (workspaceRef.current) workspaceRef.current.inert = mobileOpen }, [mobileOpen])
  useEffect(() => {
    const desktop = window.matchMedia?.('(min-width: 801px)')
    const resize = () => { if (desktop?.matches) closeMobile() }
    desktop?.addEventListener('change', resize)
    return () => desktop?.removeEventListener('change', resize)
  }, [closeMobile])
  useEffect(() => {
    const focusSearch = (event: KeyboardEvent) => {
      if (!event.defaultPrevented && !event.isComposing && !event.shiftKey && canSearch && (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k' && !document.querySelector('[role="dialog"]')) {
        event.preventDefault(); searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', focusSearch, true)
    return () => window.removeEventListener('keydown', focusSearch, true)
  }, [canSearch])
  return <div className="app-shell">
    <button type="button" className="shell-skip" onClick={() => document.getElementById('workspace-content')?.focus()}>跳转到内容</button>
    <Sidebar mobileOpen={mobileOpen} onMobileClose={closeMobile} />
    {mobileOpen && <button className="shell-nav-backdrop" aria-label="关闭导航遮罩" onClick={() => setMobileOpen(false)} tabIndex={-1} />}
    <div className="shell-workspace" ref={workspaceRef}>
      <header className="shell-topbar">
        <button className="shell-icon shell-mobile-menu" aria-label="打开导航" aria-controls="app-navigation" aria-expanded={mobileOpen} onClick={() => setMobileOpen(true)}><Menu size={23} /></button>
        <form className="shell-search" role="search" aria-label="搜索店铺" onSubmit={event => {
          event.preventDefault()
          if (!canSearch) return
          const next = new URLSearchParams(location.search)
          next.set('q', draft); next.set('page', '1')
          navigate({ pathname: '/stores', search: next.toString() })
        }}>
          <Search size={22} aria-hidden="true" />
          <input ref={searchRef} aria-label="全局搜索店铺" placeholder="搜索店铺…" value={draft} disabled={!canSearch} title={canSearch ? undefined : '返回店铺工作台后搜索'} onChange={event => setDraft(event.target.value)} />
          <kbd>Ctrl K</kbd>
        </form>
        <div className="shell-top-actions">
          <Link className="shell-icon shell-notifications" to="/notifications" aria-label={unread ? `通知中心，${unread} 条未读通知` : '通知中心'}><Bell size={24} />{unread > 0 && <span className="shell-unread" />}</Link>
          <Link className="shell-icon" to="/system/docs" aria-label="帮助中心"><HelpCircle size={25} /></Link>
          <Link className="shell-date" to="/settings"><time>{new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date())}</time><span>本机时间</span></Link>
        </div>
      </header>
      <main id="workspace-content" tabIndex={-1} className="shell-main">{children}</main>
    </div>
  </div>
}
