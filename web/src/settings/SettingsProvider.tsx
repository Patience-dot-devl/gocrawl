import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getSettings, saveSettings } from '../api/settings'
import { applyTheme } from '../theme'
import type { CrawlDefaults, GUISettings, SettingsResponse } from '../types/settings'
import { SettingsContext, type SettingsState } from './context'

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

// Loads ~/.gocrawl/gui.json through the API once, applies its theme (dark until it answers),
// and shares it with the New crawl form and the Settings view. Nothing goes to localStorage:
// the desktop shell serves the UI from a new port on every launch.
export default function SettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<SettingsResponse | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    applyTheme('dark')
    getSettings()
      .then((s) => {
        if (cancelled) return
        setSettings(s)
        applyTheme(s.theme)
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(message(e))
      })
    return () => {
      cancelled = true
    }
  }, [])

  const save = useCallback(async (next: GUISettings) => {
    const saved = await saveSettings(next)
    setSettings(saved)
    setError('')
    applyTheme(saved.theme)
    return saved
  }, [])

  const theme = settings?.theme ?? 'dark'
  const saveDefaults = useCallback(
    (defaults: CrawlDefaults) => save({ theme, crawl_defaults: defaults }),
    [save, theme],
  )

  const value = useMemo<SettingsState>(
    () => ({ settings, error, save, saveDefaults }),
    [settings, error, save, saveDefaults],
  )
  return <SettingsContext value={value}>{children}</SettingsContext>
}
