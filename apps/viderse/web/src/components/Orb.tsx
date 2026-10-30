// Orb của trợ lý — bọc AgentOrb của kit nhưng vẽ bằng gradient của Viderse
// (--vv-orb-* trong styles/viderse.css, đúng artboard .o-scout/.o-director/…).
// Kit giữ gradient riêng cho Noverse (--orb-writer kết tím-hồng), nên màu phải
// lấy từ token app, không lấy từ token kit.
// Chỉ dùng cho 4 trợ lý; avatar kênh/đối thủ không bao giờ dùng orb (ChannelAvatar).
import type { CSSProperties } from 'react'
import { AgentOrb } from '@noverse/ui'
import { ASSISTANT_ORB, assistantName, type Assistant } from './agents'

const ORB_BG: Record<Assistant, string> = {
  scout: 'var(--vv-orb-scout)',
  director: 'var(--vv-orb-director)',
  artist: 'var(--vv-orb-artist)',
  qa: 'var(--vv-orb-qa)',
}

export function Orb({ of, size = 18, live = false, dim = false, standalone = false, style, className }: {
  of: Assistant
  size?: number
  /** đang chạy → thở 2,8s */
  live?: boolean
  dim?: boolean
  /** đứng một mình → role=img với tên trợ lý */
  standalone?: boolean
  style?: CSSProperties
  className?: string
}) {
  return (
    <AgentOrb
      agent={ASSISTANT_ORB[of]}
      size={size}
      live={live}
      dim={dim}
      label={standalone ? assistantName(of) : undefined}
      // display: inline-block vì AgentOrb chỉ đặt width/height (inline thì bỏ
      // qua kích thước) — trong flex item vẫn được blockify như cũ.
      style={{ display: 'inline-block', backgroundImage: ORB_BG[of], ...style }}
      className={className}
    />
  )
}
