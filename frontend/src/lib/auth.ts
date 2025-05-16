import { writable } from 'svelte/store'

export type User = { id: number; role: string; token: string } | null

function createAuth() {
  // Initialize from localStorage
  const stored = localStorage.getItem('jwt')
  const { subscribe, set } = writable<User>(
    stored ? { id: 0, role: '', token: stored } : null
  )

  return {
    subscribe,
    login: (token: string, id: number, role: string) => {
      localStorage.setItem('jwt', token)
      set({ token, id, role })
    },
    logout: () => {
      localStorage.removeItem('jwt')
      set(null)
    }
  }
}

export const auth = createAuth()
