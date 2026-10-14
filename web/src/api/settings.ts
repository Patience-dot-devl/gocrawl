import type { GUISettings, SettingsResponse } from '../types/settings'
import { postJSON, request } from './client'

export function getSettings(): Promise<SettingsResponse> {
  return request<SettingsResponse>('/api/settings')
}

export function saveSettings(settings: GUISettings): Promise<SettingsResponse> {
  return postJSON<SettingsResponse>('/api/settings', settings, 'PUT')
}
