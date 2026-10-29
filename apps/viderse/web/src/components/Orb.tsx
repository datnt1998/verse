// Orb của trợ lý — bọc AgentOrb của kit theo ánh xạ Viderse (components/agents).
// Chỉ dùng cho 4 trợ lý; avatar kênh/đối thủ không bao giờ dùng orb (ChannelAvatar).
import type { CSSProperties } from 'react'
import { AgentOrb } from '@noverse/ui'
import { ASSISTANT_ORB, assistantName, type Assistant } from './agents'

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
      style={style}
      className={className}
    />
  )
}
