// RequestInit and RequestInfo are provided by the DOM lib

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
// simple wrapper so we write one line instead of four every time
export async function apiJSON<T = any>(input: RequestInfo, init: RequestInit = {}) {
  const res = await apiFetch(input, init)
  if (!res.ok) throw new Error((await res.json()).error ?? res.statusText)
  return res.json() as Promise<T>
}
