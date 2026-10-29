// D12 — nhóm “Não truyện” của Tuỳ chỉnh › Nâng cao (Settings.dc.html advGroups[1]): hai nút mở kho ghi chú
// bằng Obsidian trên máy chủ (của truyện, của dự án) và hàng “Sửa dữ kiện”. Không hiện đường dẫn,
// không có cài đặt embedding; “Não hệ thống” chuyển sang nhóm Hệ thống (D15, đợt 3).
import { useState } from 'react'
import { ApiError, openNaoVault } from '../../../api/client'
import { t } from '../../../i18n/vi'
import { toast } from '@noverse/ui'
import type { SettingsContext } from '../types'
import { AdvButton, AdvRow } from './AdvGroup'

export function NaoRows({ ctx }: { ctx: SettingsContext }) {
  const [opening, setOpening] = useState<'nao-story' | 'nao-project' | null>(null)
  const open = async (target: 'nao-story' | 'nao-project') => {
    setOpening(target)
    try {
      await openNaoVault(target === 'nao-story'
        ? { target, project: ctx.project, story: ctx.story }
        : { target, project: ctx.project })
      toast.ok(t.adv.nao.opened)
    } catch (cause) {
      // 404 “Chưa có kho ghi chú …” và 403 loopback: nguyên văn câu của máy chủ.
      toast.err(cause instanceof ApiError ? cause.message : t.adv.nao.failed)
    } finally { setOpening(null) }
  }
  const hasStory = !!ctx.project && !!ctx.story
  return <>
    <AdvRow label={t.adv.nao.storyLabel} value={t.adv.nao.storyValue}>
      <AdvButton pending={opening === 'nao-story'} disabled={!hasStory} title={hasStory ? undefined : 'Chọn một truyện trước'}
        onClick={() => void open('nao-story')}>{t.adv.nao.open}</AdvButton>
    </AdvRow>
    <AdvRow label={t.adv.nao.projectLabel} value={t.adv.nao.projectValue}>
      <AdvButton pending={opening === 'nao-project'} disabled={!ctx.project}
        onClick={() => void open('nao-project')}>{t.adv.nao.open}</AdvButton>
    </AdvRow>
    <AdvRow label={t.adv.nao.factsLabel} value={t.adv.nao.factsValue} />
  </>
}
