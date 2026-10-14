import type { HeaderProfile, HeaderProfileList } from '../types/profiles'
import { postJSON, request } from './client'

function profilePath(name: string): string {
  return `/api/header-profiles/${encodeURIComponent(name)}`
}

export function listHeaderProfiles(): Promise<HeaderProfileList> {
  return request<HeaderProfileList>('/api/header-profiles')
}

export function getHeaderProfile(name: string): Promise<HeaderProfile> {
  return request<HeaderProfile>(profilePath(name))
}

export function saveHeaderProfile(name: string, body: string): Promise<void> {
  return postJSON<void>(profilePath(name), { body }, 'PUT')
}

export function deleteHeaderProfile(name: string): Promise<void> {
  return request<void>(profilePath(name), { method: 'DELETE' })
}
