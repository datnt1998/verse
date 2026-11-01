import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@noverse/ui/tokens.stylex'
import './styles/viderse.stylex'
import { applyViderseLabels } from './kitLabels'
import './styles/viderse.css'
// Brand Viderse (design 02 §2): bảng token nền tối của kit + đỏ/cam Viderse đã
// sinh sẵn thành CSS (:root) bằng scripts/brand-css.ts — entry không phải nạp
// bảng token của kit và toán màu chỉ để ghi biến lúc chạy. Nạp sau viderse.css
// để :root của brand thắng :root mặc định của kit.
import './styles/brand.css'
import App from './App'

// Nhãn tiếng Việt của kit phải gắn trước lần render đầu (ConfirmDialog mở ngay).
applyViderseLabels()
if ('serviceWorker' in navigator) void navigator.serviceWorker.register('/sw.js')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
