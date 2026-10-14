// Theme switching. styles/tokens.css defines the palettes on :root[data-theme=...]: dark (also
// the default before any attribute is set), light, and system, which follows the operating
// system's prefers-color-scheme. This module only sets the attribute. The Settings view
// stores the choice server-side (gui.json), never in localStorage: the desktop shell serves
// the UI from a different port on every launch, so localStorage would start empty each time.

export type Theme = 'dark' | 'light' | 'system'

export function isTheme(value: unknown): value is Theme {
  return value === 'dark' || value === 'light' || value === 'system'
}

export function applyTheme(t: Theme): void {
  document.documentElement.dataset.theme = t
}
