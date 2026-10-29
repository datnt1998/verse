import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/base.css'
import '@noverse/ui/tokens.stylex'
import '@noverse/ui/kit.css'
import { KIT_LABELS_VI, applyBrand, defineBrand, setKitLabels } from '@noverse/ui'
import App from './App'

// Brand Viderse: một màu chính, kit tự triển thang tông → vai trò ngữ nghĩa.
// Nhãn kit tiếng Việt, tên thương hiệu riêng (BrandLockup đọc getKitLabels().brand).
setKitLabels({ ...KIT_LABELS_VI, brand: { name: 'Viderse', studio: 'Studio' } })
applyBrand(defineBrand({ name: 'viderse', primary: '#F43F5E' }))

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
