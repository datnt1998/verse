// Trang lịch sử luồng viết (#/p/:p/s/:s/work?view=run) — Gói AS mục 1: “Lịch sử ›” mở trang này
// chứ không cuộn khung Luồng viết trực tiếp lên đầu. Cùng địa chỉ với bản điện thoại (M1b).
import { Icon } from '@noverse/ui'
import { paths } from '../../router'
import { RunHistory } from './RunHistory'

export function RunHistoryPage({ project, story }: { project: string; story: string }) {
  return <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex' }}>
    <main className="scroll" style={{ flex: 1, minWidth: 0, minHeight: 0 }}>
      <div style={{ maxWidth: 920, margin: '0 auto', padding: '32px 24px 64px', display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <a href={`#${paths.workspace(project, story)}`} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--color-fg-3)' }}>
            <Icon name="arrow-left-2" size={13} />Bàn viết
          </a>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.01em' }}>Lịch sử luồng viết</h1>
          <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>Mọi vòng của truyện, lưu lại toàn bộ. Mở thẻ để xem tham số chính và kết quả.</span>
        </div>
        <RunHistory project={project} story={story} />
      </div>
    </main>
  </div>
}
