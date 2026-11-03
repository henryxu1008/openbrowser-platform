import { useEffect, useId, useRef, useState, useSyncExternalStore } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Archive, Bell, Bookmark, BookOpen, Bot, ChevronDown, ChevronLeft, ChevronRight, Cpu, FileText, Globe, Home, Layers, Monitor, MoreHorizontal, Puzzle, Settings, Tag, Users, X, type LucideIcon } from 'lucide-react'
import { authSnapshot, getBusinessApp, subscribeAuth } from '../../auth/session'
import { GetAppConfig } from '../../auth/bridge'
import { navigationConfig, projectConfig } from '../../config'
import { useLayoutStore } from '../../store/layoutStore'
import { useShellSlots } from './ShellSlots'
import brandImage from '../../resources/images/shuomi-v4-brand.png'

const icons: Record<string, LucideIcon> = { Archive, Bell, Bookmark, BookOpen, Bot, Cpu, FileText, Globe, Layers, Monitor, Puzzle, Settings, Tag }
const items = navigationConfig.flatMap(section => section.items)
const activePath = (current: string, path: string) => current === path || current.startsWith(path + '/')

function NavigationLink({ path, label, collapsed }: { path: string; label?: string; collapsed: boolean }) {
  const location = useLocation(), item = items.find(entry => entry.path === path)
  if (!item) return null
  const Icon = path === '/stores' ? Home : icons[item.icon] || Layers
  const active = activePath(location.pathname, path)
  return <Link to={path} className={`shell-nav-item${active ? ' active' : ''}`} aria-current={active ? 'page' : undefined} title={collapsed ? label || item.name : undefined}>
    <Icon size={23} aria-hidden="true" /><span className="shell-nav-label">{label || item.name}</span>
  </Link>
}

function NavigationGroup({ label, icon: Icon, paths, collapsed }: { label: string; icon: LucideIcon; paths: string[]; collapsed: boolean }) {
  const location = useLocation(), id = useId()
  const active = paths.some(path => activePath(location.pathname, path))
  const [expanded, setExpanded] = useState(active)
  useEffect(() => { if (active) setExpanded(true) }, [active])
  return <div className="shell-nav-group">
    <button className={`shell-nav-item${active ? ' active' : ''}`} aria-expanded={expanded && !collapsed} aria-controls={id} title={collapsed ? label : undefined} onClick={() => {
      if (collapsed) { useLayoutStore.getState().setSidebarCollapsed(false); setExpanded(true) }
      else setExpanded(value => !value)
    }}><Icon size={23} aria-hidden="true" /><span className="shell-nav-label">{label}</span><ChevronDown className="shell-group-chevron" size={15} /></button>
    {expanded && !collapsed && <div id={id} className="shell-nav-children">{paths.map(path => <NavigationLink key={path} path={path} collapsed={false} />)}{label === '更多工具' && <p className="shell-shortcut-hint">快速启动 <kbd>Ctrl Shift K</kbd></p>}</div>}
  </div>
}

export function Sidebar({ mobileOpen = false, onMobileClose = () => {} }: { mobileOpen?: boolean; onMobileClose?: () => void }) {
  const { sidebarCollapsed, toggleSidebar } = useLayoutStore()
  const collapsed = sidebarCollapsed && !mobileOpen
  const slots = useShellSlots(), status = useSyncExternalStore(subscribeAuth, authSnapshot, authSnapshot)
  const [appVersion, setAppVersion] = useState('')
  const aside = useRef<HTMLElement>(null)
  const focusEpoch = useRef(0)
  useEffect(() => {
    let live = true
    if (getBusinessApp()?.GetAppConfig) void GetAppConfig().then(config => {
      if (live) setAppVersion(typeof config?.version === 'string' ? config.version.trim() : '')
    }).catch(() => { if (live) setAppVersion('') })
    return () => { live = false }
  }, [])
  useEffect(() => {
    if (!mobileOpen) return
    const epoch = ++focusEpoch.current
    const surface = aside.current, previous = document.activeElement as HTMLElement | null
    const controls = () => Array.from(surface?.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),select:not([disabled]),input:not([disabled]),summary') || []).filter(item => {
      // Chromium can report rectangles for absolute children of closed
      // details even though they are outside the sequential focus order.
      const closed = item.closest('details:not([open])')
      return item.getClientRects().length && (!closed || item === closed.querySelector('summary'))
    })
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); onMobileClose() }
      if (event.key !== 'Tab') return
      const all = controls(), first = all[0], last = all[all.length - 1]
      if (!first) { event.preventDefault(); surface?.focus() }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    surface?.querySelector<HTMLElement>('[aria-label="关闭导航"]')?.focus()
    surface?.addEventListener('keydown', keydown)
    return () => {
      surface?.removeEventListener('keydown', keydown)
      // Layout clears background inert in the same passive-effect flush.
      // Restore afterwards, unless a newer navigation opening owns focus.
      queueMicrotask(() => { if (focusEpoch.current === epoch && previous?.isConnected) previous.focus() })
    }
  }, [mobileOpen, onMobileClose])
  const subject = status.subject || '当前用户'
  return <aside ref={aside} id="app-navigation" role={mobileOpen ? 'dialog' : undefined} aria-modal={mobileOpen || undefined} aria-label="应用导航" tabIndex={-1} className={`shell-sidebar${collapsed ? ' shell-sidebar-collapsed' : ''}${mobileOpen ? ' shell-sidebar-open' : ''}`}>
    <div className="shell-brand">
      <Link to="/stores" aria-label={projectConfig.name}><img src={brandImage} alt="硕米指纹标识" /><span className="shell-brand-name"><strong>硕米</strong><small>SHUOMI</small></span></Link>
      <button className="shell-icon shell-mobile-close" aria-label="关闭导航" onClick={onMobileClose}><X size={22} /></button>
    </div>
    <div className="shell-organization"><Users size={22} aria-hidden="true" /><div ref={slots?.setOrganizationTarget} className="shell-organization-slot" /><Link className="shell-organization-fallback" to="/stores">选择企业<ChevronDown size={14} /></Link></div>
    <nav className="shell-nav" aria-label="主要导航">
      <h2 className="shell-nav-section">日常运营</h2>
      <NavigationLink path="/stores" collapsed={collapsed} />
      <h2 className="shell-nav-section shell-resources">资源管理</h2>
      <NavigationLink path="/browser/list" label="浏览器环境" collapsed={collapsed} />
      <NavigationLink path="/browser/proxy-pool" label="网络资源" collapsed={collapsed} />
      <NavigationGroup label="内核与扩展" icon={Layers} paths={['/browser/cores', '/browser/extensions']} collapsed={collapsed} />
      <NavigationGroup label="更多工具" icon={MoreHorizontal} paths={['/browser/automation', '/browser/bookmarks', '/browser/tags', '/browser/logs', '/system/docs', '/notifications']} collapsed={collapsed} />
    </nav>
    <div className="shell-nav-bottom">
      <NavigationGroup label="设置与备份" icon={Settings} paths={['/settings', '/system/backup']} collapsed={collapsed} />
      <details className="shell-account" onKeyDown={event => {
        if (event.key === 'Escape') { event.preventDefault(); event.currentTarget.open = false; event.currentTarget.querySelector('summary')?.focus() }
      }}>
        <summary title={sidebarCollapsed ? subject : undefined}><span className="shell-avatar" aria-hidden="true">{subject.slice(0, 1).toUpperCase()}</span><span className="shell-account-name"><strong>{subject}</strong><small>已登录</small></span><ChevronRight size={17} /></summary>
        <div className="shell-account-panel"><div ref={slots?.setAccountTarget} /><Link to="/profile">关于应用</Link>{appVersion && <small>版本 {appVersion}</small>}</div>
      </details>
      <button className="shell-collapse" onClick={toggleSidebar} aria-label={sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'}>{sidebarCollapsed ? <ChevronRight size={17} /> : <><ChevronLeft size={17} /><span>收起侧边栏</span></>}</button>
    </div>
  </aside>
}
