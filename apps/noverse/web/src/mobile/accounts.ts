import { useCallback, useEffect, useState } from 'react'
import { logout } from '../api/client'
import type { User } from '../api/types'
import { getRemembered, rememberAccount as rememberStored } from '../api/storage'
import { loginPublic, pairPublic } from '../screens/auth/publicApi'
import { cache, events } from '../data'
import { accountTransition } from './session'
import { setOutboxAccount } from './outbox'

let account = ''
const drafts = new Map<string, Map<string, string>>()
const listeners = new Set<() => void>()
let acceptUser: ((user: User | null) => void | Promise<void>) | null = null
export const readAccounts = getRemembered
export function rememberAccount(user: User) {
  rememberStored({ username: user.username, name: user.name || user.username, role: user.role, lastLoginAt: new Date().toISOString() })
}
export function setDraftAccount(username: string) { account = username; listeners.forEach((listener) => listener()) }
export function registerAccountSession(handler: (user: User | null) => void | Promise<void>) {
  acceptUser = handler
  return () => { if (acceptUser === handler) acceptUser = null }
}
export function getDraft(key: string) { return drafts.get(account)?.get(key) ?? '' }
export function latestDraft() { return [...(drafts.get(account)?.values() ?? [])].filter(Boolean).at(-1) ?? '' }
export function setDraft(key: string, text: string) {
  let own = drafts.get(account)
  if (!own) { own = new Map(); drafts.set(account, own) }
  own.set(key, text)
  listeners.forEach((listener) => listener())
}
export function useAccountDraft(key: string): [string, (text: string) => void] {
  const [value, setValue] = useState(() => getDraft(key))
  useEffect(() => {
    const update = () => setValue(getDraft(key))
    update(); listeners.add(update)
    return () => { listeners.delete(update) }
  }, [key])
  const updateDraft = useCallback((text: string) => setDraft(key, text), [key])
  return [value, updateDraft]
}
export async function beginAccountSwitch() {
  const accept = acceptUser
  accountTransition.begin()
  setOutboxAccount('')
  try { await logout(); events.stop(); cache.clear(); await accept?.(null) }
  finally { accountTransition.finish(false) }
}
/** One cookie session: revoke it before authenticating another account; never persist credentials. */
export async function switchAccount(credentials: { username: string; password: string } | { code: string }) {
  const accept = acceptUser
  accountTransition.begin()
  setOutboxAccount('')
  try {
    await logout()
    events.stop(); cache.clear()
    const result = 'code' in credentials ? await pairPublic(credentials.code) : await loginPublic(credentials.username, credentials.password)
    rememberAccount(result.user)
    setDraftAccount(result.user.username)
    setOutboxAccount(result.user.username)
    await accept?.(result.user)
    accountTransition.finish(true)
    events.start()
    void cache.revalidateActive(0)
    return result
  } catch (error) {
    await accept?.(null)
    accountTransition.finish(false)
    throw error
  }
}
