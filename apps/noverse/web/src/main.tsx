import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/base.css'
import '@noverse/ui/tokens.stylex'
import { KIT_LABELS_VI, setKitLabels } from '@noverse/ui'
import App from './App'
import { registerMobilePWA } from './mobile/pwa'
import { installDeepLinks } from './mobile/deepLinks'

setKitLabels(KIT_LABELS_VI)

registerMobilePWA()
installDeepLinks()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
