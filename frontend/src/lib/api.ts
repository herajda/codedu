import { get } from 'svelte/store'
import type { RequestInit, RequestInfo } from 'node-fetch'
import { auth } from './auth'

export async function apiFetch(
  input: RequestInfo,
  init: RequestInit = {}
) {
  const token =
    localStorage.getItem('jwt') || get(auth)?.token || undefined
  const headers = new Headers(init.headers)
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const res = await fetch(input, { ...init, headers })
  return res
}
// simple wrapper so we write one line instead of four every time
export async function apiJSON<T = any>(input: RequestInfo, init: RequestInit = {}) {
  const res = await apiFetch(input, init)
  if (!res.ok) throw new Error((await res.json()).error ?? res.statusText)
  return res.json() as Promise<T>
}
