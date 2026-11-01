import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@noverse/ui/tokens.stylex'
import '@noverse/ui/kit.css'
import { PHONE_MAX_VIDERSE, setPhoneMax } from '@noverse/ui'
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
// Kit dùng chung mặc định mốc điện thoại 640 (Noverse); Viderse là 767 (chưa có artboard máy tính bảng).
setPhoneMax(PHONE_MAX_VIDERSE)
if ('serviceWorker' in navigator) void navigator.serviceWorker.register('/sw.js')

const start = () =>
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
start()
