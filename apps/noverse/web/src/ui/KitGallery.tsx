import { useState } from 'react'
import { BrandMark } from '@noverse/ui'
import { ConfirmGallery } from './ConfirmGallery'
import { type ConfirmSituation } from './confirmPresets'
import { MenusGallery } from './MenusGallery'
import { OverlaysGallery } from './OverlaysGallery'
import { SelectGallery } from './SelectGallery'
import { InputsGallery } from './InputsGallery'
import { BrandGallery } from './BrandGallery'
import { Tabs } from '@noverse/ui'
import { ToastHost } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import '@noverse/ui/kit.css'

// Kit.dc.html › tabsDef: icon 15 px, tím khi tab đang chọn (Tabs variant="bar" lo phần màu).
const tabs = [['confirm', 'Xác nhận', 'shield-tick'], ['menus', 'Menu & tài khoản', 'user'], ['select', 'Select & Combobox', 'arrow-down-2'], ['inputs', 'Ô nhập & nút', 'edit-2'], ['overlays', 'Gợi ý & trạng thái', 'info-circle'], ['brand', 'Thương hiệu', 'lamp-on']]
  .map(([id, label, icon]) => ({ id, label, icon: <Icon name={icon} size={15} /> }))

export function KitGallery() {
  const query = new URLSearchParams(location.hash.split('?')[1])
  const [section, setSection] = useState(query.get('section') ?? 'confirm')
  const [situation, setSituation] = useState<ConfirmSituation>((query.get('confirm') as ConfirmSituation) || 'user')
  return <div className="kit-gallery"><header><div style={{ display: 'flex', alignItems: 'center', gap: 10, flex: '0 0 auto' }}><BrandMark size={22} /><span style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.25 }}><span style={{ fontSize: 14, fontWeight: 600 }}>Bộ thành phần</span><span style={{ fontSize: 11.5, color: '#7E7E8A' }}>Mọi thứ đều bấm thử được</span></span></div><Tabs variant="bar" items={tabs} value={section} onChange={setSection} label="Nhóm thành phần" style={{ maxWidth: 760, margin: '0 auto' }} /><span style={{ flex: '0 0 auto', fontSize: 12, color: '#7E7E8A', whiteSpace: 'nowrap' }}>Token: <span className="mono" style={{ color: '#A0A0AB' }}>03-tokens.md</span></span></header><main>
    {section === 'confirm' && <ConfirmGallery situation={situation} onSituationChange={setSituation} />}
    {section === 'menus' && <MenusGallery onDanger={() => { setSituation('story'); setSection('confirm') }} />}
    {section === 'select' && <SelectGallery initialOpen={query.get('open') ?? 'voice'} />}
    {section === 'inputs' && <InputsGallery />}
    {section === 'overlays' && <OverlaysGallery />}
    {section === 'brand' && <BrandGallery />}
  </main><ToastHost /></div>
}
