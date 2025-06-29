// RequestInit and RequestInfo are provided by the DOM lib
export async function apiFetch(
  input: RequestInfo,
  init: RequestInit = {},
  _retry = false
) {
  const res = await fetch(input, {
    ...init,
    credentials: 'include'
  })
  if (res.status === 401 && !_retry) {
    const r = await fetch('/api/refresh', { method: 'POST', credentials: 'include' })
    if (r.ok) return apiFetch(input, init, true)
  }
  return res
}
// simple wrapper so we write one line instead of four every time
export async function apiJSON<T = any>(input: RequestInfo, init: RequestInit = {}) {
  const res = await apiFetch(input, init)
  if (!res.ok) throw new Error((await res.json()).error ?? res.statusText)
  return res.json() as Promise<T>
}
