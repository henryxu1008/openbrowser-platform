import { useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserCore, BrowserSettings } from '../../types'
import {
  fetchBrowserCores,
  fetchBrowserSettings,
  saveBrowserSettings,
} from '../../api'

const DEFAULT_BROWSER_SETTINGS: BrowserSettings = {
  userDataRoot: 'data',
  defaultFingerprintArgs: [],
  defaultLaunchArgs: [],
  defaultStartUrls: [],
  lightStartEnabled: true,
  restoreLastSession: false,
  startReadyTimeoutMs: 3000,
  startStableWindowMs: 1200,
  defaultConnectorType: 'xray',
}

export function useBrowserListSettings() {
  const [settingsModalOpen, setSettingsModalOpen] = useState(false)
  const [settings, setSettings] = useState<BrowserSettings>(DEFAULT_BROWSER_SETTINGS)
  const [fingerprintText, setFingerprintText] = useState('')
  const [launchText, setLaunchText] = useState('')
  const [startUrlsText, setStartUrlsText] = useState('')
  const [savingSettings, setSavingSettings] = useState(false)

  const [cores, setCores] = useState<BrowserCore[]>([])
  const loadSettings = async () => {
    const data = await fetchBrowserSettings()
    setSettings(data)
    setFingerprintText((data.defaultFingerprintArgs || []).join('\n'))
    setLaunchText((data.defaultLaunchArgs || []).join('\n'))
    setStartUrlsText((data.defaultStartUrls || []).join('\n'))
  }

  const loadCores = async () => {
    setCores(await fetchBrowserCores())
  }

  const handleOpenSettings = async () => {
    await Promise.all([loadSettings(), loadCores()])
    setSettingsModalOpen(true)
  }

  const handleSaveSettings = async () => {
    setSavingSettings(true)
    try {
      await saveBrowserSettings({
        ...settings,
        defaultFingerprintArgs: fingerprintText.split('\n').map(s => s.trim()).filter(Boolean),
        defaultLaunchArgs: launchText.split('\n').map(s => s.trim()).filter(Boolean),
        defaultStartUrls: startUrlsText.split('\n').map(s => s.trim()).filter(Boolean),
      })
      toast.success('配置已保存')
      setSettingsModalOpen(false)
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSavingSettings(false)
    }
  }

  return {
    settingsModalOpen,
    setSettingsModalOpen,
    settings,
    setSettings,
    fingerprintText,
    setFingerprintText,
    launchText,
    setLaunchText,
    startUrlsText,
    setStartUrlsText,
    savingSettings,
    cores,
    loadCores,
    handleOpenSettings,
    handleSaveSettings,
  }
}
