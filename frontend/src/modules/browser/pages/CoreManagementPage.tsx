import { useState } from 'react'
import { Card } from '../../../shared/components'
import { KernelVersionPicker } from '../components/KernelVersionPicker'

export function CoreManagementPage() {
  const [version, setVersion] = useState('')
  return <div className="space-y-4">
    <h1 className="text-xl font-semibold">内核版本</h1>
    <Card>
      <p className="mb-4 text-sm text-[var(--color-text-secondary)]">选择平台发行的可用版本。准备完成后，可在实例或店铺配置中选择使用。</p>
      <KernelVersionPicker value={version} onChange={setVersion} scopeKey="kernel-versions" />
    </Card>
  </div>
}
