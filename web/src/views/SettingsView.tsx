import { useCallback, useEffect, useState } from 'react'
import { listAnalyzers } from '../api/crawls'
import { deleteHeaderProfile, getHeaderProfile, listHeaderProfiles, saveHeaderProfile } from '../api/profiles'
import CrawlFields from '../components/crawlform/CrawlFields'
import { fromParams, toParams, type FormValues } from '../components/crawlform/values'
import Button from '../components/ui/Button'
import ConfirmDialog from '../components/ui/ConfirmDialog'
import Notice from '../components/ui/Notice'
import { useSettings } from '../settings/context'
import { applyTheme, type Theme } from '../theme'
import type { AnalyzerInfo } from '../types/crawls'
import type { CrawlDefaults } from '../types/settings'
import './SettingsView.css'

// Same rule as config.headerProfileName on the server; checked here only for a quicker message.
const PROFILE_NAME = /^[A-Za-z0-9_-][A-Za-z0-9._-]*$/

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export default function SettingsView() {
  const { settings, error: loadError } = useSettings()
  // Nothing below may save until the settings answered (or failed): saving earlier would
  // write the built-in defaults over the file.
  const locked = settings === null && !loadError
  const [profiles, setProfiles] = useState<string[]>([])
  const [profilesError, setProfilesError] = useState('')

  const reloadProfiles = useCallback(async () => {
    try {
      const r = await listHeaderProfiles()
      setProfiles(r.profiles)
      setProfilesError('')
    } catch (e) {
      setProfilesError(message(e))
    }
  }, [])

  useEffect(() => {
    listHeaderProfiles()
      .then((r) => setProfiles(r.profiles))
      .catch((e: unknown) => setProfilesError(message(e)))
  }, [])

  return (
    <div className="set">
      <h1>Settings</h1>
      {loadError && (
        <Notice tone="error">
          Couldn't load your settings: {loadError}. Saving any setting below replaces the settings file.
        </Notice>
      )}
      <ThemeSection locked={locked} />
      <DefaultsSection profiles={profiles} locked={locked} />
      <ProfilesSection profiles={profiles} loadError={profilesError} onChanged={reloadProfiles} />
      <section className="set-panel">
        <h2>Crawl store</h2>
        <p>
          Saved crawls live in <code>{settings?.store_dir ?? '…'}</code>. Start gocrawl with{' '}
          <code>gocrawl serve --store-dir &lt;dir&gt;</code> to use another folder.
        </p>
      </section>
    </div>
  )
}

function ThemeSection({ locked }: { locked: boolean }) {
  const { settings, save } = useSettings()
  const [status, setStatus] = useState('')
  const theme: Theme = settings?.theme ?? 'dark'

  async function choose(next: Theme) {
    applyTheme(next)
    setStatus('')
    try {
      await save({ theme: next, crawl_defaults: settings?.crawl_defaults ?? {} })
      setStatus('Saved.')
    } catch (e) {
      applyTheme(theme)
      setStatus(`Couldn't save the theme: ${message(e)}`)
    }
  }

  return (
    <section className="set-panel">
      <h2>Theme</h2>
      <label className="cf-field">
        <span className="cf-label">Theme</span>
        <select value={theme} disabled={locked} onChange={(e) => void choose(e.target.value as Theme)}>
          <option value="dark">Dark</option>
          <option value="light">Light</option>
          <option value="system">Match the system</option>
        </select>
      </label>
      {status && <p role="status">{status}</p>}
    </section>
  )
}

function DefaultsSection({ profiles, locked }: { profiles: string[]; locked: boolean }) {
  const { settings, save } = useSettings()
  const defaults = settings?.crawl_defaults
  const [values, setValues] = useState<FormValues>(() => fromParams(defaults))
  const [seenDefaults, setSeenDefaults] = useState(defaults)
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  const [dirty, setDirty] = useState(false)
  // Follow the saved defaults (first load, and the server's stripped copy after each save),
  // but never overwrite edits that were not saved yet, such as when the theme changes.
  if (defaults !== seenDefaults) {
    setSeenDefaults(defaults)
    if (!dirty) setValues(fromParams(defaults))
  }

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e: unknown) => setError(message(e)))
  }, [])

  async function store(crawlDefaults: CrawlDefaults, done: string) {
    setError('')
    setNote('')
    try {
      await save({ theme: settings?.theme ?? 'dark', crawl_defaults: crawlDefaults })
      setDirty(false)
      setValues(fromParams(crawlDefaults))
      setNote(done)
    } catch (e) {
      setError(message(e))
    }
  }

  return (
    <form
      className="set-panel cf"
      onSubmit={(e) => {
        e.preventDefault()
        void store(toParams(values, 'defaults'), 'Saved. The New crawl form starts from these values.')
      }}
    >
      <h2>Crawl defaults</h2>
      <p>These values pre-fill the New crawl form.</p>
      <fieldset className="set-lock" disabled={locked}>
        <CrawlFields
          values={values}
          onChange={(patch) => {
            setDirty(true)
            setNote('')
            setValues((v) => ({ ...v, ...patch }))
          }}
          mode="defaults"
          analyzers={analyzers}
          profiles={profiles}
        />
        {error && <Notice tone="error">{error}</Notice>}
        {note && <Notice tone="info">{note}</Notice>}
        <div className="cf-actions">
          <Button type="submit" variant="primary">
            Save defaults
          </Button>
          <Button type="button" variant="secondary" onClick={() => void store({}, 'Reset to the built-in defaults.')}>
            Reset to built-in defaults
          </Button>
        </div>
      </fieldset>
    </form>
  )
}

interface Editing {
  name: string
  body: string
  isNew: boolean
}

function ProfilesSection({
  profiles,
  loadError,
  onChanged,
}: {
  profiles: string[]
  loadError: string
  onChanged: () => Promise<void>
}) {
  const [editing, setEditing] = useState<Editing | null>(null)
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  async function edit(name: string) {
    setError('')
    setNote('')
    try {
      const p = await getHeaderProfile(name)
      setEditing({ name: p.name, body: p.body, isNew: false })
    } catch (e) {
      setError(message(e))
    }
  }

  function startNew() {
    setError('')
    setNote('')
    setEditing({ name: '', body: '', isNew: true })
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    if (!editing) return
    const name = editing.name.trim()
    setError('')
    setNote('')
    if (!PROFILE_NAME.test(name)) {
      setError("Use letters, digits, '.', '_' and '-' for the name, not starting with '.'.")
      return
    }
    if (editing.isNew && profiles.includes(name)) {
      setError(`A profile named "${name}" already exists. Pick another name, or edit it from the list.`)
      return
    }
    try {
      await saveHeaderProfile(name, editing.body)
      setEditing({ name, body: editing.body, isNew: false })
      setNote(`Saved profile "${name}".`)
      await onChanged()
    } catch (err) {
      setError(message(err))
    }
  }

  async function handleDelete() {
    setConfirmingDelete(false)
    if (!editing || editing.isNew) return
    setError('')
    try {
      await deleteHeaderProfile(editing.name)
      setNote(`Deleted profile "${editing.name}".`)
      setEditing(null)
      await onChanged()
    } catch (err) {
      setError(message(err))
    }
  }

  return (
    <section className="set-panel">
      <h2>Header profiles</h2>
      <p>
        Request headers stored on this machine in <code>~/.gocrawl/headers</code>, for example a
        Shopify crawler access signature. A crawl picks one by name, so the values never pass
        through the form.
      </p>
      {loadError && <Notice tone="error">{loadError}</Notice>}
      {profiles.length === 0 ? (
        <p>No header profiles yet. Create one for headers you send on every crawl of a site.</p>
      ) : (
        <ul className="set-profiles">
          {profiles.map((name) => (
            <li key={name}>
              <Button type="button" variant="secondary" onClick={() => void edit(name)} aria-pressed={editing?.name === name}>
                {name}
              </Button>
            </li>
          ))}
        </ul>
      )}
      <Button type="button" variant="secondary" onClick={startNew}>
        New profile
      </Button>

      {editing && (
        <form className="set-editor" onSubmit={handleSave}>
          <label className="cf-field">
            <span className="cf-label">Name</span>
            <input
              type="text"
              value={editing.name}
              readOnly={!editing.isNew}
              onChange={(e) => setEditing({ ...editing, name: e.target.value })}
              placeholder="uk"
            />
          </label>
          <label className="cf-field">
            <span className="cf-label">Headers (one &quot;Name: value&quot; per line)</span>
            <span className="cf-hint">Lines starting with # are comments.</span>
            <textarea
              rows={6}
              spellCheck={false}
              value={editing.body}
              onChange={(e) => setEditing({ ...editing, body: e.target.value })}
              placeholder={'# UK store\nSignature: sig1=:...:'}
            />
          </label>
          <div className="cf-actions">
            <Button type="submit" variant="primary">
              Save profile
            </Button>
            {!editing.isNew && (
              <Button type="button" variant="stop" onClick={() => setConfirmingDelete(true)}>
                Delete profile
              </Button>
            )}
            <Button type="button" variant="secondary" onClick={() => setEditing(null)}>
              Close
            </Button>
          </div>
        </form>
      )}
      {error && <Notice tone="error">{error}</Notice>}
      {note && <Notice tone="info">{note}</Notice>}
      {confirmingDelete && editing && (
        <ConfirmDialog
          title="Delete header profile?"
          confirmLabel="Delete profile"
          cancelLabel="Keep profile"
          confirmVariant="stop"
          onConfirm={() => void handleDelete()}
          onCancel={() => setConfirmingDelete(false)}
        >
          <p>Delete "{editing.name}"? Crawls that use it fail until you pick another profile.</p>
        </ConfirmDialog>
      )}
    </section>
  )
}
