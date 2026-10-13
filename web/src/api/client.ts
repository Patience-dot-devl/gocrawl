// Shared fetch helpers for every API module (api/<feature>.ts). A non-2xx response throws an
// Error carrying the server's {"error": "..."} message when there is one, so views can show
// it as-is.

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new Error(body?.error || `${res.status} ${res.statusText}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// postJSON sends body as JSON. The server's guard insists on application/json for JSON
// routes (a 415 otherwise), so always go through here rather than a bare fetch.
export function postJSON<T>(path: string, body: unknown, method: string = 'POST'): Promise<T> {
  return request<T>(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

// postForm sends a multipart upload. No Content-Type header: the browser sets
// multipart/form-data with the boundary itself.
export function postForm<T>(path: string, form: FormData): Promise<T> {
  return request<T>(path, { method: 'POST', body: form })
}
