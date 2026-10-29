import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@noverse/ui/tokens.stylex'
import { applyViderseLabels } from './kitLabels'
import { applyViderseBrand } from './brand'
import './styles/tokens.css'
import './styles/base.css'
import './styles/viderse.css'
import './styles/components.css'
import App from './App'

// Nhãn tiếng Việt của kit phải gắn trước lần render đầu (ConfirmDialog mở ngay).
// Brand Viderse ghi đè token nền sáng lên nền tối của kit (design 02 §2).
applyViderseLabels()
applyViderseBrand()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
