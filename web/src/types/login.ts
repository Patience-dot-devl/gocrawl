export type LoginState = 'waiting' | 'captured' | 'closed' | 'expired'

// GET /api/login-sessions/{id}. Never carries a cookie value.
export interface LoginSession {
  id: string
  state: LoginState
  host: string
  cookie_names: string[]
  cookie_count: number
  expires_at?: string
}

export interface LoginAvailability {
  available: boolean
  reason?: string
}
