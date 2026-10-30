import { Link } from 'react-router-dom'
import { Archive, ChevronRight, ChevronUp, LayoutGrid, List, Play, RefreshCw, Sliders, Trash2, Upload } from 'lucide-react'

import { Button, Card, FormItem, Input, Modal, Switch, Textarea } from '../../../shared/components'

import type { BrowserCore, BrowserGroupWithCount, BrowserProxy, BrowserSettings } from '../types'
import { InstanceFilterBar } from './InstanceFilterBar'
import type { InstanceFilters } from './InstanceFilterBar'

export type BrowserViewMode = 'card' | 'table'

interface BrowserListHeaderProps {
  profileCount: number
  filteredProfileCount: number
  runningCount: number
  headerCollapsed: boolean
  viewMode: BrowserViewMode
  proxies: BrowserProxy[]
  cores: BrowserCore[]
  groups: BrowserGroupWithCount[]
  allTags: string[]
  filters: InstanceFilters
  onFiltersChange: (next: InstanceFilters) => void
  onToggleHeaderCollapsed: () => void
  onRefresh: () => void
  onOpenSettings: () => void
  onOpenTrash: () => void
  onImportProfiles: () => void
  importingProfiles?: boolean
  onViewModeChange: (next: BrowserViewMode) => void
}

export function BrowserListHeader({
  profileCount,
  filteredProfileCount,
  runningCount,
  headerCollapsed,
  viewMode,
  proxies,
  cores,
  groups,
  allTags,
  filters,
  onFiltersChange,
  onToggleHeaderCollapsed,
  onRefresh,
  onOpenSettings,
  onOpenTrash,
  onImportProfiles,
  importingProfiles = false,
  onViewModeChange,
}: BrowserListHeaderProps) {
  const statItems = [
    { label: '总数', value: profileCount },
    { label: '运行', value: runningCount },
    { label: '停止', value: Math.max(0, profileCount - runningCount) },
  ]

  return (
    <Card padding="none" className="shadow-[var(--shadow-sm)]">
      <div className="flex flex-col gap-3 px-4 py-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
          <h1 className="shrink-0 text-xl font-semibold text-[var(--color-text-primary)]">实例列表</h1>
          <div className="flex flex-wrap items-center gap-2">
            {statItems.map((item) => (
              <div
                key={item.label}
                className="inline-flex h-8 items-center gap-2 rounded-lg bg-[var(--color-bg-muted)] px-3 text-sm"
              >
                <span className="text-[var(--color-text-muted)]">{item.label}</span>
                <span className="font-semibold text-[var(--color-text-primary)]">{item.value}</span>
              </div>
            ))}
            {filteredProfileCount !== profileCount && (
              <div className="inline-flex h-8 items-center gap-2 rounded-lg bg-[var(--color-accent)]/10 px-3 text-sm">
                <span className="text-[var(--color-text-muted)]">筛选</span>
                <span className="font-semibold text-[var(--color-accent)]">{filteredProfileCount}</span>
              </div>
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 xl:justify-end">
          <Button variant="secondary" size="sm" onClick={onToggleHeaderCollapsed}>
            {headerCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronUp className="w-4 h-4" />}
            {headerCollapsed ? '展开筛选' : '收起筛选'}
          </Button>
          <Button variant="secondary" size="sm" onClick={onRefresh}>
            <RefreshCw className="w-4 h-4" />刷新
          </Button>
          <Button variant="secondary" size="sm" onClick={onOpenSettings}>
            <Sliders className="w-4 h-4" />基础配置
          </Button>
          <Button variant="secondary" size="sm" onClick={onOpenTrash}>
            <Trash2 className="w-4 h-4" />回收站
          </Button>
          <Button variant="secondary" size="sm" onClick={onImportProfiles} loading={importingProfiles}>
            <Upload className="w-4 h-4" />导入实例
          </Button>
          <Link to="/system/backup">
            <Button variant="secondary" size="sm" title="打开全局备份与恢复">
              <Archive className="w-4 h-4" />全局备份
            </Button>
          </Link>

          <div className="flex items-center rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] p-0.5" role="group" aria-label="视图切换">
            <button
              type="button"
              aria-label="卡片视图"
              aria-pressed={viewMode === 'card'}
              className={`rounded-md p-1.5 text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text-primary)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] ${viewMode === 'card' ? 'bg-[var(--color-bg-surface)] text-[var(--color-accent)] shadow-sm' : ''}`}
              onClick={() => onViewModeChange('card')}
              title="卡片视图"
            >
              <LayoutGrid className="w-4 h-4" />
            </button>
            <button
              type="button"
              aria-label="表格视图"
              aria-pressed={viewMode === 'table'}
              className={`rounded-md p-1.5 text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text-primary)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] ${viewMode === 'table' ? 'bg-[var(--color-bg-surface)] text-[var(--color-accent)] shadow-sm' : ''}`}
              onClick={() => onViewModeChange('table')}
              title="表格视图"
            >
              <List className="w-4 h-4" />
            </button>
          </div>

          <span className="mx-1 hidden h-5 w-px bg-[var(--color-border-muted)] sm:block" />
          <Link to="/browser/edit/new" className="shrink-0">
            <Button size="sm">
              <Play className="w-4 h-4" />新建配置
            </Button>
          </Link>
        </div>
      </div>

      {!headerCollapsed && (
        <div className="border-t border-[var(--color-border-muted)] px-4 py-3">
          <InstanceFilterBar
            filters={filters}
            onChange={onFiltersChange}
            proxies={proxies}
            cores={cores}
            allTags={allTags}
            groups={groups}
          />
        </div>
      )}
    </Card>
  )
}

interface BrowserListSettingsModalProps {
  open: boolean
  settings: BrowserSettings
  fingerprintText: string
  launchText: string
  startUrlsText: string
  savingSettings: boolean
  onClose: () => void
  onSave: () => void
  onSettingsChange: (patch: Partial<BrowserSettings>) => void
  onFingerprintTextChange: (next: string) => void
  onLaunchTextChange: (next: string) => void
  onStartUrlsTextChange: (next: string) => void
}

export function BrowserListSettingsModal({
  open,
  settings,
  fingerprintText,
  launchText,
  startUrlsText,
  savingSettings,
  onClose,
  onSave,
  onSettingsChange,
  onFingerprintTextChange,
  onLaunchTextChange,
  onStartUrlsTextChange,
}: BrowserListSettingsModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="基础配置"
      width="700px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={onSave} loading={savingSettings}>保存</Button>
        </>
      }
    >
      <div className="space-y-4">
        <Link to="/browser/cores" onClick={onClose} className="text-sm underline">选择平台内核版本</Link>

        <FormItem label="用户数据根目录">
          <Input
            value={settings.userDataRoot}
            onChange={(event) => onSettingsChange({ userDataRoot: event.target.value })}
            placeholder="data"
          />
        </FormItem>
        <FormItem label="默认指纹参数" hint="每行一个参数">
          <Textarea
            value={fingerprintText}
            onChange={(event) => onFingerprintTextChange(event.target.value)}
            rows={3}
            placeholder="--fingerprint-brand=Chrome"
          />
        </FormItem>
        <FormItem label="默认启动参数" hint="每行一个参数">
          <Textarea
            value={launchText}
            onChange={(event) => onLaunchTextChange(event.target.value)}
            rows={3}
            placeholder="--disable-sync"
          />
        </FormItem>
        <FormItem label="默认启动页面" hint="每行一个 URL，留空则启动时不再自动打开页面">
          <Textarea
            value={startUrlsText}
            onChange={(event) => onStartUrlsTextChange(event.target.value)}
            rows={4}
            placeholder="启动 URL"
          />
        </FormItem>
        <FormItem label="轻启动模式" hint="先起空白页，实例就绪后再打开默认页面">
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] px-3 py-2">
            <span className="text-sm text-[var(--color-text-primary)]">延后打开启动页</span>
            <Switch
              checked={settings.lightStartEnabled}
              onChange={(checked) => onSettingsChange({ lightStartEnabled: checked })}
            />
          </div>
        </FormItem>
        <FormItem label="默认恢复历史标签" hint="实例选择跟随内核时使用；不影响启动页和启动书签。实例未覆盖时，下次启动恢复之前的标签页和窗口。">
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] px-3 py-2">
            <div>
              <p className="text-sm text-[var(--color-text-primary)]">内核默认</p>
            </div>
            <Switch
              checked={settings.restoreLastSession}
              onChange={(checked) => onSettingsChange({ restoreLastSession: checked })}
            />
          </div>
        </FormItem>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 items-end">
          <FormItem label="启动就绪超时（毫秒）">
            <Input
              type="number"
              min={1000}
              step={500}
              value={settings.startReadyTimeoutMs}
              onChange={(event) =>
                onSettingsChange({
                  startReadyTimeoutMs: Math.max(1000, Number(event.target.value) || 3000),
                })
              }
              placeholder="3000"
            />
          </FormItem>
          <FormItem label="启动稳定窗口（毫秒）">
            <Input
              type="number"
              min={0}
              step={100}
              value={settings.startStableWindowMs}
              onChange={(event) =>
                onSettingsChange({
                  startStableWindowMs: Math.max(0, Number(event.target.value) || 1200),
                })
              }
              placeholder="1200"
            />
          </FormItem>
        </div>
      </div>
    </Modal>
  )
}
