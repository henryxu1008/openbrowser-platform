import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

// DOM placement only. Authentication and Store owners retain their existing
// state, admission checks, requests and callbacks through these portals.
interface Slots {
  organizationTarget: HTMLElement | null
  accountTarget: HTMLElement | null
  setOrganizationTarget: (target: HTMLElement | null) => void
  setAccountTarget: (target: HTMLElement | null) => void
}
const Context = createContext<Slots | null>(null)
export const useShellSlots = () => useContext(Context)
export function ShellSlotsProvider({ children }: { children: ReactNode }) {
  const [organizationTarget, setOrganizationTarget] = useState<HTMLElement | null>(null)
  const [accountTarget, setAccountTarget] = useState<HTMLElement | null>(null)
  const slots = useMemo(() => ({ organizationTarget, accountTarget, setOrganizationTarget, setAccountTarget }), [organizationTarget, accountTarget])
  return <Context.Provider value={slots}>{children}</Context.Provider>
}
export function ShellAccountControls({ children }: { children: ReactNode }) {
  const slots = useShellSlots()
  return slots?.accountTarget ? createPortal(children, slots.accountTarget) : <>{children}</>
}
export function ShellOrganization({ children }: { children: ReactNode }) {
  const slots = useShellSlots()
  return slots?.organizationTarget
    ? createPortal(<div className="store-workbench shell-organization-control">{children}</div>, slots.organizationTarget)
    : <>{children}</>
}
