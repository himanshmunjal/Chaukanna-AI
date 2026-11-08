// Who is using the dashboard: the family, a demo visitor, or someone who must log in.

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { api, LOGGED_OUT_EVENT, type Access } from './api'
import { LoginPage } from '../pages/LoginPage'

// If the server can't say (an older backend, say), behave as the local dashboard always has.
// The server still decides what each request may do.
const OPEN: Access = { family: true, demo: false, login_required: false, can_login: false, logged_in: false }

interface AccessValue {
  access: Access
  logout: () => void
  goToLogin: () => void
}

const AccessContext = createContext<AccessValue>({ access: OPEN, logout: () => {}, goToLogin: () => {} })

export function AccessProvider({ children }: { children: ReactNode }) {
  const [access, setAccess] = useState<Access | null>(null)
  const [wantsLogin, setWantsLogin] = useState(false)

  const refresh = useCallback(() => {
    api.me().then(setAccess).catch(() => setAccess(OPEN))
  }, [])

  useEffect(() => {
    refresh()
    window.addEventListener(LOGGED_OUT_EVENT, refresh)
    return () => window.removeEventListener(LOGGED_OUT_EVENT, refresh)
  }, [refresh])

  if (!access) return <div className="min-h-screen bg-slate-950" />

  if (access.login_required || (wantsLogin && !access.family)) {
    return (
      <LoginPage
        onCancel={access.login_required ? undefined : () => setWantsLogin(false)}
        onLoggedIn={() => {
          setWantsLogin(false)
          refresh()
        }}
      />
    )
  }

  const logout = () => {
    api.logout().catch(() => {}).finally(refresh)
  }
  return (
    <AccessContext.Provider key={String(access.family)} value={{ access, logout, goToLogin: () => setWantsLogin(true) }}>
      {children}
    </AccessContext.Provider>
  )
}

export function useAccess(): AccessValue {
  return useContext(AccessContext)
}
