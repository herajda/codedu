import { writable } from 'svelte/store'
import { apiFetch } from './api'

export type User = { id: number; role: string; token: string } | null

function createAuth() {
  const saved = localStorage.getItem('user')
  const initial: User = saved ? JSON.parse(saved) : null
  const { subscribe, set } = writable<User>(initial)

  /* ── Public API ───────────────────────────────────────────── */
  return {
    subscribe,

    /** Called from Login.svelte after successful auth */
    login(token: string, id: number, role: string) {
      const user = { token, id, role }
      localStorage.setItem('user', JSON.stringify(user))
      set(user)
    },

    /** Log out everywhere */
    logout() {
      localStorage.removeItem('user')
      set(null)
    },

    /** Run once at start-up: restore role/id when only a token is stored */
    async init() {
      const cur = JSON.parse(localStorage.getItem('user') || 'null')
      if (!cur?.token) return                      // nothing to do
      if (cur.role) { set(cur); return }           // already complete

      // We have a token but no metadata – fetch /api/me
      const r = await apiFetch('/api/me')
      if (r.ok) {
        const me = await r.json()
        this.login(cur.token, me.id, me.role)      // fills the blanks
      } else {
        // token expired → wipe it
        this.logout()
      }
    }
  }
}

export const auth = createAuth()
