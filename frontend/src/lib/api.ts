import { get } from 'svelte/store'
import type { RequestInit, RequestInfo } from 'node-fetch'
import { auth } from './auth'

export async function apiFetch(
  input: RequestInfo,
  init: RequestInit = {}
) {
  const token = localStorage.getItem('jwt')
  const headers = new Headers(init.headers)
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const res = await fetch(input, { ...init, headers })
  return res
}
