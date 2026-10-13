import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/tokens.css'
import './styles/fonts.css'
import './index.css'
import App from './App.tsx'
import { applyTheme } from './theme'

// Dark until the Settings feature loads the saved theme from the server.
applyTheme('dark')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
