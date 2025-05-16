import { get } from 'svelte/store'
import type { RequestInit, RequestInfo } from 'node-fetch'
import { auth } from './auth'

export async function apiFetch(
  input: RequestInfo,
  init: RequestInit = {}
) {
  const user = get(auth)
  const headers = new Headers(init.headers)
  if (user?.token) headers.set('Authorization', `Bearer ${user.token}`)
  const res = await fetch(input, { ...init, headers })
  return res
}
