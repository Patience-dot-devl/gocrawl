import type { Theme } from '../theme'
import type { StartCrawlParams } from './crawls'

// crawlrequest.Params as stored in ~/.gocrawl/gui.json: every field optional, never a secret.
// The server strips bearer_token, basic_auth, cookie, inline headers, and user:password in
// URLs before saving.
export type CrawlDefaults = Partial<Omit<StartCrawlParams, 'save' | 'login_session'>>

export interface GUISettings {
  theme: Theme
  crawl_defaults: CrawlDefaults
}

// GET/PUT /api/settings response: the stored settings plus the read-only store directory.
export interface SettingsResponse extends GUISettings {
  store_dir: string
}
