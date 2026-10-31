import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Tab Hồ sơ kênh (artboard Channel › tab=settings): khối nhận diện, "Thông tin
// kênh" (tên, link YouTube — tự lưu khi rời ô), "Khi viết tiêu đề, mô tả, tag"
// (ngôn ngữ, giọng văn, đoạn cuối mô tả đếm ≤1.000, tag luôn thêm ≤10), hai lối
// tắt sang Nguồn/Template và vùng xoá kênh. Lỗi 400 theo trường hiện dưới ô;
// chữ đang gõ được giữ nguyên để người dùng sửa rồi lưu lại.
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Field, FieldError, Icon, Segmented, TextArea } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton } from '../../components'
import { confirmDeleteChannel } from '../../components/confirm'
import { t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { navigate, paths } from '../../router'
import { channelFieldErrors, patchChannel, deleteChannel, type Channel, type ChannelLanguage } from '../../api/channels'
import { IdentityBlock } from './IdentityBlock'

type SaveState = 'idle' | 'saving' | 'saved' | 'error'

/** Lưu một trường khi rời ô; giữ nguyên chữ đang gõ nếu server từ chối. */
function useFieldSave(value: string, onSave: (draft: string) => Promise<void>) {
  const [draft, setDraft] = useState(value)
  const origin = useRef(value)
  const [state, setState] = useState<SaveState>('idle')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (value === origin.current) return
    origin.current = value
    setDraft(value)
  }, [value])

  const commit = async () => {
    if (draft === origin.current) return
    setState('saving')
    setError(null)
    try {
      await onSave(draft)
      origin.current = draft
      setState('saved')
    } catch (err) {
      setState('error')
      setError(err instanceof Error ? err.message : t.channels.settings.saveError)
    }
  }

  return { draft, setDraft, state, error, commit }
}

const stateLabel = (state: SaveState): string | null => {
  if (state === 'saving') return t.channels.settings.saving
  if (state === 'saved') return t.channels.settings.saved
  return null
}

function SaveHint({ state }: { state: SaveState }) {
  const label = stateLabel(state)
  return (
    <span className={`vch-saveline ${stylex.props(channelStyles.saveline).className}`} aria-live="polite">
      <Icon name={state === 'saved' ? 'tick-b' : 'clock'} size={12} />
      {label ?? t.channels.settings.autosave}
    </span>
  )
}

export function SettingsTab({ channel }: { channel: Channel }) {
  const tt = t.channels.settings
  const apply = (next: Channel) => {
    setData(keys.channel(channel.id), next)
    void invalidate(keys.channels())
  }

  const save = async (patch: Parameters<typeof patchChannel>[1], field?: string): Promise<Channel> => {
    try {
      const next = await patchChannel(channel.id, patch)
      apply(next)
      return next
    } catch (err) {
      const fields = channelFieldErrors(err)
      const message = (field && fields[field]) || (err instanceof Error ? err.message : tt.saveError)
      throw new Error(message)
    }
  }

  const name = useFieldSave(channel.name, async (draft) => { await save({ name: draft }, 'name') })
  const url = useFieldSave(channel.youtubeUrl ?? '', async (draft) => {
    await save({ youtubeUrl: draft.trim() ? draft.trim() : null }, 'youtubeUrl')
  })
  const styleNote = useFieldSave(channel.styleNote ?? '', async (draft) => {
    await save({ styleNote: draft.trim() ? draft : null }, 'styleNote')
  })
  const footer = useFieldSave(channel.descriptionFooter ?? '', async (draft) => {
    await save({ descriptionFooter: draft.trim() ? draft : null }, 'descriptionFooter')
  })

  const [language, setLanguage] = useState<ChannelLanguage>(channel.language)
  useEffect(() => { setLanguage(channel.language) }, [channel.language])
  const [tags, setTags] = useState<string[]>(channel.alwaysTags)
  useEffect(() => { setTags(channel.alwaysTags) }, [channel.alwaysTags])
  const [tagDraft, setTagDraft] = useState('')
  const [tagError, setTagError] = useState<string | null>(null)

  const saveTags = async (next: string[]) => {
    setTagError(null)
    setTags(next)
    try {
      await save({ alwaysTags: next }, 'alwaysTags')
    } catch (err) {
      setTagError(err instanceof Error ? err.message : tt.saveError)
    }
  }

  const addTag = () => {
    const tag = tagDraft.trim()
    if (!tag) return
    if (tags.length >= 10) { setTagError(tt.tagLimit); return }
    if (tags.includes(tag)) { setTagDraft(''); return }
    setTagDraft('')
    void saveTags([...tags, tag])
  }

  const onDelete = () => {
    void confirmDeleteChannel({
      name: channel.name || t.channels.page.noName,
      runningCount: 0,
      consequences: [tt.deleteConsequence],
      onConfirm: async () => {
        await deleteChannel(channel.id)
        void invalidate(keys.channels())
        navigate(paths.workbench())
      },
    })
  }

  return (
    <>
      <IdentityBlock channel={channel} />

      <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={tt.infoTitle}>
        <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
          <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="user" size={16} />{tt.infoTitle}</h2>
        </div>
        <div className={`vch-form ${stylex.props(channelStyles.form).className}`} style={{ padding: 0 }}>
          <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
            <FieldRow label={tt.name} htmlFor="vch-name" right={<SaveHint state={name.state} />}>
              <Field className={stylex.props(channelStyles.touchTarget).className}
                id="vch-name"
                value={name.draft}
                placeholder={tt.namePlaceholder}
                maxLength={200}
                invalid={name.error ? true : undefined}
                onChange={(event) => name.setDraft(event.target.value)}
                onBlur={() => void name.commit()}
                onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur() }}
              />
            </FieldRow>
            {name.error ? <FieldError>{name.error}</FieldError> : <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.noNameHint}</p>}
          </div>

          <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
            <FieldRow label={tt.url} htmlFor="vch-url" right={<SaveHint state={url.state} />}>
              <Field className={stylex.props(channelStyles.touchTarget).className}
                id="vch-url"
                type="url"
                value={url.draft}
                placeholder={tt.urlPlaceholder}
                maxLength={2048}
                invalid={url.error ? true : undefined}
                onChange={(event) => url.setDraft(event.target.value)}
                onBlur={() => void url.commit()}
                onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur() }}
              />
            </FieldRow>
            {url.error ? <FieldError>{url.error}</FieldError> : <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.urlHint}</p>}
          </div>
        </div>
      </section>

      <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={tt.writingTitle}>
        <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
          <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="magicpen" size={16} />{tt.writingTitle}</h2>
        </div>
        <div className={`vch-form ${stylex.props(channelStyles.form).className}`} style={{ padding: 0 }}>
          <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
            <span className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`}>{tt.language}</span>
            <Segmented buttonStyle={channelStyles.touchTarget}
              label={tt.language}
              size="sm"
              fit="content"
              value={language}
              options={[
                { id: 'en', label: t.channels.page.language.en },
                { id: 'vi', label: t.channels.page.language.vi },
                { id: 'auto', label: t.channels.page.language.auto },
              ]}
              onChange={(id) => {
                const next = id as ChannelLanguage
                setLanguage(next)
                void save({ language: next }, 'language').catch(() => setLanguage(channel.language))
              }}
            />
            <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.languageHint}</p>
          </div>

          <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
            <FieldRow label={tt.styleNote} htmlFor="vch-style" right={<SaveHint state={styleNote.state} />}>
              <TextArea
                id="vch-style"
                value={styleNote.draft}
                placeholder={tt.styleNotePlaceholder}
                maxLength={8000}
                showCount={false}
                rows={3}
                onChange={(event) => styleNote.setDraft(event.target.value)}
                onBlur={() => void styleNote.commit()}
              />
            </FieldRow>
            {styleNote.error && <FieldError>{styleNote.error}</FieldError>}
          </div>

          <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
            <FieldRow label={tt.footer} htmlFor="vch-footer" right={<SaveHint state={footer.state} />}>
              <TextArea
                id="vch-footer"
                value={footer.draft}
                placeholder={tt.footerPlaceholder}
                maxLength={1000}
                rows={3}
                onChange={(event) => footer.setDraft(event.target.value)}
                onBlur={() => void footer.commit()}
              />
            </FieldRow>
            {footer.error ? <FieldError>{footer.error}</FieldError> : <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.footerBadge} · {tt.footerHint}</p>}
          </div>

          <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
            <span className={`vch-field__row ${stylex.props(channelStyles.fieldRow).className}`}>
              <span className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`}>{tt.tags} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.tagsBadge}</span></span>
              <span className={`vch-counter num ${stylex.props(channelStyles.counter).className}`}>{tt.tagsCount(tags.length, 10)}</span>
            </span>
            <div className={`vch-tags ${stylex.props(channelStyles.tags).className}`}>
              {tags.map((tag) => (
                <span className={`vch-tag ${stylex.props(channelStyles.tag).className}`} key={tag}>
                  {tag}
                  <button {...stylex.props(channelStyles.tagButton)}
                    type="button"
                    aria-label={tt.removeTag(tag)}
                    onClick={() => void saveTags(tags.filter((item) => item !== tag))}
                  >
                    <Icon name="close" size={11} />
                  </button>
                </span>
              ))}
              <input
                className={`vch-taginput ${stylex.props(channelStyles.taginput).className}`}
                value={tagDraft}
                placeholder={tt.tagPlaceholder}
                aria-label={tt.addTag}
                maxLength={60}
                disabled={tags.length >= 10}
                onChange={(event) => setTagDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') { event.preventDefault(); addTag() }
                }}
              />
            </div>
            {tagError ? <FieldError>{tagError}</FieldError> : null}
            <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.writingHint}</p>
          </div>
        </div>
      </section>

      <nav className={`vch-navrows ${stylex.props(channelStyles.navrows).className}`} aria-label={tt.infoTitle}>
        <a className={`vch-navrow ${stylex.props(channelStyles.navrow).className}`} href={`#${paths.channel(channel.id, 'sources')}`}>
          <Icon name="radar" size={16} />
          <span className={`vch-navrow__text ${stylex.props(channelStyles.navrowText).className}`}>
            <span>{tt.linkSources}</span>
            <span className={`vch-navrow__sub ${stylex.props(channelStyles.navrowSub).className}`}>{tt.linkSourcesSub(channel.sources.length, channel.pacePerDay)}</span>
          </span>
          <Icon name="arrow-r" size={14} />
        </a>
        <a className={`vch-navrow ${stylex.props(channelStyles.navrow).className}`} href={`#${paths.channel(channel.id, 'templates')}`}>
          <Icon name="layer" size={16} />
          <span className={`vch-navrow__text ${stylex.props(channelStyles.navrowText).className}`}>
            <span>{tt.linkTemplates}</span>
            <span className={`vch-navrow__sub ${stylex.props(channelStyles.navrowSub).className}`}>{tt.linkTemplatesSub(channel.templates.length)}</span>
          </span>
          <Icon name="arrow-r" size={14} />
        </a>
      </nav>

      <section className={`vch-delete ${stylex.props(channelStyles.delete).className}`} aria-label={tt.deleteTitle}>
        <h2 {...stylex.props(channelStyles.deleteTitle)}>{tt.deleteTitle}</h2>
        <p {...stylex.props(channelStyles.deleteBody)}>{tt.deleteBody}</p>
        <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
          <ActionButton className={stylex.props(channelStyles.touchTarget).className} variant="danger-soft" size="sm" onClick={onDelete} icon={<Icon name="trash" size={14} />}>
            {tt.deleteButton}
          </ActionButton>
        </div>
      </section>
    </>
  )
}

/** Nhãn một dòng + chỗ hiện trạng thái lưu bên phải. */
function FieldRow({ label, htmlFor, right, children }: {
  label: string
  htmlFor?: string
  right?: ReactNode
  children: ReactNode
}) {
  return (
    <>
      <span className={`vch-field__row ${stylex.props(channelStyles.fieldRow).className}`}>
        <label className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`} htmlFor={htmlFor}>{label}</label>
        {right}
      </span>
      {children}
    </>
  )
}
