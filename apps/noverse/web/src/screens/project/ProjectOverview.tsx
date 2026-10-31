// Tổng quan dự án (Outline.dc.html › view project): tên + mô tả · 4 số liệu · Truyện · Ý tưởng · Tư liệu · ô ghi nhanh ý tưởng.
import { useEffect, useRef, useState, type DragEvent } from 'react'
import { canManageProject, createIdea, uploadProjectAsset, uploadProjectVoice } from '../../api/client'
import { useSession } from '../../api/session'
import type { Idea, ProjectDetail, StorySummary } from '../../api/types'
import { keys, useAction, useConnection } from '../../data'
import { paths, useRoute } from '../../router'
import { projectLabel, storyLabel } from '../../shell/labels'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { relativeTime } from '../auth/parts'
import { CHIP_NEUTRAL, CHIP_PURPLE, Chip, Dock, StoryContextMenu } from './bits'
import { STORY_STATUS_LABEL, formatWords } from './format'
import { settingsLink } from './links'
import { ScrollTopButton } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { RulesPanel } from '../settings/sections/rules'
import { ProjectStyleSection } from './tabs/ProjectStyleSection'

function StoryTile({ project, story }: { project: string; story: StorySummary }) {
  if (story.status === 'idea') {
    return (
      <div className="card pj-tile" style={{ borderStyle: 'dashed', background: 'transparent', justifyContent: 'center' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}><Chip tone={CHIP_NEUTRAL}>{STORY_STATUS_LABEL.idea}</Chip></span>
        <span style={{ fontSize: 15, fontWeight: 600, color: '#C9C9D1' }}>{storyLabel(story)}</span>
        <span style={{ fontSize: 12.5, color: '#7E7E8A' }}>Chưa bắt đầu viết · tạo từ một ý tưởng bên dưới</span>
      </div>
    )
  }
  const writing = story.status === 'writing'
  const ratio = story.totalChapters > 0 ? Math.min(story.completedChapters / story.totalChapters, 1) : 0
  const summary = [
    `${story.volumes} tập`,
    `${story.totalChapters} chương`,
    `${formatWords(story.totalWords)} chữ`,
    story.totalChapters > 0 && story.completedChapters < story.totalChapters ? `đã viết ${story.completedChapters}/${story.totalChapters}` : '',
    story.exportsCount > 0 ? `đã có ${story.exportsCount} bản xuất` : '',
  ].filter(Boolean).join(' · ')
  return (
    <StoryContextMenu project={project} story={story}>
      <a className="card pj-tile hov-row" href={`#${paths.story(project, story.story)}`} style={{ border: '1px solid rgba(255,255,255,0.07)', cursor: 'pointer', color: '#EDEDEF' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Chip tone={writing ? CHIP_PURPLE : CHIP_NEUTRAL}>
            {writing && <AgentOrb agent="writer" size={8} live={story.running} dim={!story.running} />}
            {STORY_STATUS_LABEL[story.status]}
          </Chip>
          {story.tags[0] && <Chip tone={CHIP_NEUTRAL}>{story.tags[0]}</Chip>}
        </span>
        <span style={{ fontSize: 15, fontWeight: 600, lineHeight: 1.4 }}>{storyLabel(story)}</span>
        <span style={{ height: 4, borderRadius: 3, background: 'rgba(255,255,255,0.07)', overflow: 'hidden' }}>
          <span style={{ display: 'block', height: '100%', width: `${Math.round(ratio * 100)}%`, background: 'linear-gradient(90deg, #7C93F3, #9A63E3, #C9457F)' }} />
        </span>
        <span className="num" style={{ fontSize: 12.5, color: '#8E8E99' }}>{summary}</span>
      </a>
    </StoryContextMenu>
  )
}

function IdeaRow({ project, idea, first }: { project: string; idea: Idea; first: boolean }) {
  return (
    <div className="hov-row" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderTop: first ? 'none' : '1px solid rgba(255,255,255,0.05)' }}>
      <Icon name="note-text" size={16} style={{ color: '#A0A0AB' }} />
      <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.45 }}>
        <span style={{ fontSize: 14, color: '#EDEDEF' }}>{idea.title}</span>
        <span style={{ fontSize: 12.5, color: '#8E8E99', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{idea.excerpt}</span>
      </span>
      <span style={{ fontSize: 12, color: '#6E6E78', whiteSpace: 'nowrap' }}>{relativeTime(idea.createdAt)}</span>
      <a className="hov-ghost" href={`#${paths.newstory(project)}?idea=${encodeURIComponent(idea.id)}`} style={{
        height: 30, padding: '0 10px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', fontSize: 12.5,
        display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', color: '#EDEDEF',
      }}>
        <Icon name="magic-star" size={14} style={{ color: '#C9A6F5' }} />Viết thành truyện
      </a>
    </div>
  )
}

type AssetKind = 'cover' | 'music' | 'voice'
const ASSET_ZONES: { kind: AssetKind; icon: string; color: string; label: string; hint: string; accept: string }[] = [
  { kind: 'cover', icon: 'layer', color: '#C9A6F5', label: 'Bìa truyện', hint: 'Ảnh bìa cho ebook và sách nói', accept: 'image/jpeg,image/png,image/webp' },
  { kind: 'music', icon: 'music', color: '#9DB0F7', label: 'Nhạc nền', hint: 'Dùng khi ghép sách nói', accept: 'audio/*' },
  { kind: 'voice', icon: 'voice-square', color: '#A6EDC1', label: 'Giọng mẫu', hint: 'Ghi âm để bộ đọc bắt chước', accept: 'audio/*' },
]
const ADDED: Record<AssetKind, string> = { cover: 'Đã thêm bìa truyện', music: 'Đã thêm nhạc nền', voice: 'Đã thêm giọng mẫu' }

function DropZone({ project, zone }: { project: string; zone: (typeof ASSET_ZONES)[number] }) {
  const connection = useConnection()
  const input = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const upload = useAction(async (files: File[]) => {
    for (const file of files) {
      if (zone.kind === 'voice') await uploadProjectVoice(project, file.name.replace(/\.[^.]+$/, ''), file)
      else await uploadProjectAsset(project, zone.kind, file)
    }
    toast.ok(ADDED[zone.kind])
  }, { invalidate: [keys.project(project)] })
  const pick = (list: FileList | null) => { if (list && list.length > 0) void upload.run(Array.from(list)) }
  const onDrop = (event: DragEvent) => { event.preventDefault(); setOver(false); pick(event.dataTransfer.files) }
  return (
    <div className="pj-drop" data-over={over} onDragOver={(event) => { event.preventDefault(); setOver(true) }} onDragLeave={() => setOver(false)} onDrop={onDrop}>
      <Icon name={zone.icon} size={20} style={{ color: zone.color }} />
      <span style={{ fontSize: 14, fontWeight: 500 }}>{zone.label}</span>
      <span style={{ fontSize: 12.5, color: '#8E8E99' }}>{zone.hint}</span>
      <input ref={input} type="file" hidden multiple={zone.kind !== 'cover'} accept={zone.accept} aria-label={`Thêm tệp cho ${zone.label}`}
        onChange={(event) => { pick(event.target.files); event.target.value = '' }} />
      <button className="hov-ghost" type="button" disabled={upload.busy || !connection.writable} onClick={() => input.current?.click()} style={{
        marginTop: 4, height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)',
        background: 'transparent', color: '#C9C9D1', fontSize: 12,
      }}>Thêm tệp</button>
    </div>
  )
}

// Quy tắc cấp dự án dùng cùng trình sửa với tab truyện và Tuỳ chỉnh.
export function ProjectOverview({ project, detail }: { project: string; detail: ProjectDetail }) {
  const { user } = useSession()
  const [draft, setDraft] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const route = useRoute()
  useEffect(() => {
    const anchor = route.search.get('section')
    if (anchor) document.getElementById(anchor)?.scrollIntoView({ block: 'start' })
  }, [route.search])
  const saveIdea = useAction(async (text: string) => { await createIdea(project, text) }, { success: 'Đã lưu vào Ý tưởng', invalidate: [keys.project(project)] })
  const submitIdea = () => {
    const text = draft.trim()
    if (!text) return
    void saveIdea.run(text).then(() => setDraft(''))
  }
  const stats = [
    ['Truyện', String(detail.stories.length)],
    ['Đang viết', String(detail.project.runningCount)],
    ['Tổng số chữ', formatWords(detail.stories.reduce((sum, story) => sum + story.totalWords, 0))],
    ['Bản xuất', String(detail.stories.reduce((sum, story) => sum + story.exportsCount, 0))],
  ]
  const hasDraft = draft.trim() !== ''
  return (
    <main style={{ flex: '1 1 auto', minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', position: 'relative' }}>
      <div ref={scrollRef} className="scroll" style={{ flex: '1 1 auto', minHeight: 0, padding: '22px 28px 110px', display: 'flex', flexDirection: 'column', gap: 22 }}>
        <div id="overview" style={{ display: 'flex', alignItems: 'flex-end', gap: 16 }}>
          <div style={{ flex: '1 1 auto', minWidth: 0 }}>
            <h1 ref={headingRef} tabIndex={-1} style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-0.02em' }}>{projectLabel(detail.project, project)}</h1>
            <p style={{ margin: '4px 0 0', fontSize: 14, color: '#A0A0AB' }}>
              {detail.project.description ? `${detail.project.description} · ` : ''}
              {user && canManageProject(user, project) && <a href={`#${settingsLink('info', { project })}`} style={{ color: 'var(--color-accent)' }}>Sửa</a>}
            </p>
          </div>
          <a className="hov-primary" href={`#${paths.newstory(project)}`} style={{
            height: 38, padding: '0 16px 0 12px', display: 'flex', alignItems: 'center', gap: 8, borderRadius: 10,
            background: '#EDEDEF', color: '#0B0B0D', fontSize: 13.5, fontWeight: 600, whiteSpace: 'nowrap',
          }}>
            <Icon name="add" size={16} />Truyện mới
          </a>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 10 }}>
          {stats.map(([label, value]) => (
            <div key={label} className="card" style={{ borderRadius: 14, padding: '12px 14px', display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: 12, color: '#8E8E99' }}>{label}</span>
              <span className="num" style={{ fontSize: 20, fontWeight: 600 }}>{value}</span>
            </div>
          ))}
        </div>

        <section id="stories" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name="book-1" size={17} style={{ color: '#A0A0AB' }} />Truyện
          </h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 10 }}>
            {detail.stories.map((story) => <StoryTile key={story.id} project={project} story={story} />)}
          </div>
        </section>
        <section id="pstyle" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <ProjectStyleSection project={project} detail={detail} />
          <div id="prules"><RulesPanel key={project} scope="project" project={project} /></div>
        </section>
        <section id="ideas" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name="lamp-on" size={17} style={{ color: '#F6CB8C' }} />Ý tưởng{' '}
            <span className="num" style={{ fontSize: 12.5, color: '#7E7E8A', fontWeight: 400 }}>{detail.ideas.length}</span>
          </h2>
          <div className="card" style={{ borderRadius: 16, overflow: 'hidden' }}>
            {detail.ideas.length === 0
              ? <div style={{ padding: '12px 14px', fontSize: 13, color: '#8E8E99' }}>Chưa có ý tưởng nào — ghi nhanh một ý ở ô bên dưới.</div>
              : detail.ideas.map((idea, index) => <IdeaRow key={idea.id} project={project} idea={idea} first={index === 0} />)}
          </div>
        </section>

        <section id="assets" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name="box" size={17} style={{ color: '#9DB0F7' }} />Tư liệu{' '}
            <span style={{ fontSize: 12.5, color: '#7E7E8A', fontWeight: 400 }}>bìa, nhạc nền, giọng mẫu dùng chung cho các truyện</span>
          </h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 10 }}>
            {ASSET_ZONES.map((zone) => <DropZone key={zone.kind} project={project} zone={zone} />)}
          </div>
        </section>
      </div>
      <ScrollTopButton target={scrollRef} focusTarget={headingRef} label={t.scrollTop.page} size="md" style={{ right: 48, bottom: 92 }} />
      <Dock maxWidth={720} icon="lamp-on" iconColor="#F6CB8C" label="Ý tưởng mới" placeholder="Ghi nhanh một ý tưởng truyện…"
        sendLabel="Lưu ý tưởng" value={draft} onChange={setDraft} onSubmit={submitIdea}
        buttonStyle={{ background: hasDraft ? '#EDEDEF' : 'rgba(255,255,255,0.08)', color: hasDraft ? '#0B0B0D' : '#7E7E8A' }} />
    </main>
  )
}
