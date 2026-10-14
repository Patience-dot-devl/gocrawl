import type { LoginAvailability, LoginSession } from '../types/login'
import { postJSON, request } from './client'

const base = '/api/login-sessions'

function sessionPath(id: string): string {
  return `${base}/${encodeURIComponent(id)}`
}

export function loginAvailability(): Promise<LoginAvailability> {
  return request<LoginAvailability>(`${base}/available`)
}

export function startLogin(url: string): Promise<LoginSession> {
  return postJSON<LoginSession>(base, { url })
}

export function getLogin(id: string): Promise<LoginSession> {
  return request<LoginSession>(sessionPath(id))
}

export function captureLogin(id: string): Promise<LoginSession> {
  return postJSON<LoginSession>(`${sessionPath(id)}/capture`, {})
}

export function cancelLogin(id: string): Promise<void> {
  return request<void>(sessionPath(id), { method: 'DELETE' })
}
