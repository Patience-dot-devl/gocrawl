import { createContext, useContext } from 'react'
import type { CrawlDefaults, GUISettings, SettingsResponse } from '../types/settings'

export interface SettingsState {
  // null until GET /api/settings answers (and stays null if it fails; see error).
  settings: SettingsResponse | null
  error: string
  save: (next: GUISettings) => Promise<SettingsResponse>
  saveDefaults: (defaults: CrawlDefaults) => Promise<SettingsResponse>
}

export const SettingsContext = createContext<SettingsState | null>(null)

export function useSettings(): SettingsState {
  const state = useContext(SettingsContext)
  if (!state) throw new Error('useSettings must be used inside <SettingsProvider>')
  return state
}
