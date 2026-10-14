import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/tokens.css'
import './styles/fonts.css'
import './styles/base.css'
import App from './App.tsx'
import SettingsProvider from './settings/SettingsProvider'
import { applyTheme } from './theme'

// Dark until the Settings feature loads the saved theme from the server.
applyTheme('dark')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <SettingsProvider>
      <App />
    </SettingsProvider>
  </StrictMode>,
)
