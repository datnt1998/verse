import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@noverse/ui/tokens.stylex'
import '@noverse/ui/kit.css'
import { PHONE_MAX_VIDERSE, setPhoneMax } from '@noverse/ui'
import './styles/viderse.stylex'
import { applyViderseLabels } from './kitLabels'
import { applyViderseBrand } from './brand'
import './styles/viderse.css'
import App from './App'

// Nhãn tiếng Việt của kit phải gắn trước lần render đầu (ConfirmDialog mở ngay).
// Brand Viderse ghi đè token nền sáng lên nền tối của kit (design 02 §2).
applyViderseLabels()
applyViderseBrand()
// Kit dùng chung mặc định mốc điện thoại 640 (Noverse); Viderse là 767 (chưa có artboard máy tính bảng).
setPhoneMax(PHONE_MAX_VIDERSE)
if ('serviceWorker' in navigator) void navigator.serviceWorker.register('/sw.js')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
