// RequestInit and RequestInfo are provided by the DOM lib
import { browser } from '$app/environment';

function resolveInput(input: RequestInfo): RequestInfo {
  if (typeof input === 'string') {
    if (input.startsWith('/api')) {
      if (!browser) {
        // On the server, call backend directly via internal service name
        const base = (((globalThis as any).process?.env?.SSR_API_BASE) ?? 'http://backend:8080').replace(/\/$/, '')
        return base + input;
      }
      // In the browser, keep relative path so nginx routes it
      return input;
    }
  }
  return input;
}

export async function apiFetch(
  input: RequestInfo,
  init: RequestInit = {},
  _retry = false
) {
  const url = resolveInput(input)
  const res = await fetch(url, {
    ...init,
    credentials: 'include'
  })
  if (res.status === 401 && !_retry) {
    const refreshURL = browser ? '/api/refresh' : ((((globalThis as any).process?.env?.SSR_API_BASE) ?? 'http://backend:8080').replace(/\/$/, '') + '/api/refresh')
    const r = await fetch(refreshURL, { method: 'POST', credentials: 'include' })
    if (r.ok) return apiFetch(url, init, true)
  }
  return res
}
// simple wrapper so we write one line instead of four every time
export async function apiJSON<T = any>(input: RequestInfo, init: RequestInit = {}) {
  const res = await apiFetch(input, init)
  if (!res.ok) throw new Error((await res.json()).error ?? res.statusText)
  return res.json() as Promise<T>
}

// Minimal helper for SSE subscriptions used by session stream demo/clients
export function subscribeSSE(url: string, onMessage: (ev: MessageEvent) => void) {
  const es = new EventSource(url, { withCredentials: true })
  const handler = (ev: MessageEvent) => onMessage(ev)
  es.addEventListener('message', handler)
  return () => {
    es.removeEventListener('message', handler)
    es.close()
  }
}