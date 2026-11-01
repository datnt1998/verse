import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Tab Hồ sơ kênh (artboard Channel › tab=settings): board vẽ hai cột 403 px.
// Cột trái là "Thông tin kênh" (tên, link YouTube — tự lưu khi rời ô, một dòng
// trạng thái lưu ở đáy card); cột phải là "Khi viết tiêu đề, mô tả, tag" (ngôn
// ngữ, giọng văn, đoạn cuối mô tả đếm ≤1.000, tag luôn thêm ≤10, Quy tắc đã
// học) rồi ba lối tắt sang Nguồn/Template/Trợ lý AI. Vùng xoá kênh nằm dưới,
// rộng hết. Lỗi 400 theo trường hiện dưới ô; chữ đang gõ được giữ nguyên để
// người dùng sửa rồi lưu lại.
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Field, FieldError, Icon, Segmented, TextArea } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, Orb } from '../../components'
import { confirmDeleteChannel } from '../../components/confirm'
import { t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { navigate, paths } from '../../router'
import { channelFieldErrors, patchChannel, deleteChannel, type Channel, type ChannelLanguage } from '../../api/channels'
import { getAgents, listProviders } from '../../api/ai'
import { IdentityBlock } from './IdentityBlock'
import { LearnedRulesList } from '../../components/learn/LearnedRulesList'

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

/** Trạng thái lưu của một trường, hiện trong hàng nhãn (không chiếm thêm dòng). */
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

  // Dòng phụ của lối tắt "Trợ lý AI" lấy từ cấu hình thật (/api/ai/agents +
  // /api/ai/providers) như board vẽ; chưa có dữ liệu thì chỉ nói dùng chung.
  const agents = useResource(keys.aiAgents(), ({ signal }) => getAgents(signal))
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const directorModel = agents.data?.director.model.model
  const drawProvider = agents.data?.artist.draw.provider
  const drawLabel = drawProvider ? (providers.data?.find((item) => item.id === drawProvider)?.label ?? drawProvider) : ''

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

  const saving = name.state === 'saving' || url.state === 'saving'
  const failed = name.state === 'error' || url.state === 'error'
  const saved = name.state === 'saved' || url.state === 'saved'

  return (
    <>
      <IdentityBlock channel={channel} />

      <section {...stylex.props(channelStyles.settings)} aria-label={tt.sectionLabel}>
        <div {...stylex.props(channelStyles.settingsGrid)}>
          <section {...stylex.props(channelStyles.card, channelStyles.cardRoom)} aria-label={tt.infoTitle}>
            <h2 {...stylex.props(channelStyles.cardHeading, channelStyles.cardHeadingQuiet)}>
              <Icon name="setting" size={16} />{tt.infoTitle}
            </h2>
            <div {...stylex.props(channelStyles.form, channelStyles.formRoom)}>
              <div {...stylex.props(channelStyles.field)}>
                <FieldRow label={tt.name} htmlFor="vch-name">
                  <Field xstyle={channelStyles.settingsField} className={stylex.props(channelStyles.touchTarget).className}
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
                {name.error && <FieldError>{name.error}</FieldError>}
              </div>

              <div {...stylex.props(channelStyles.field)}>
                <FieldRow label={tt.url} htmlFor="vch-url">
                  <Field xstyle={channelStyles.settingsField} className={stylex.props(channelStyles.touchTarget).className}
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
                {url.error ? <FieldError>{url.error}</FieldError> : <p {...stylex.props(channelStyles.fieldHint, channelStyles.hintRoom)}>{tt.urlHint}</p>}
              </div>
            </div>
            <span {...stylex.props(channelStyles.saveline)} aria-live="polite">
              <Icon name={failed ? 'info' : saved && !saving ? 'tick-b' : 'clock'} size={14} />
              {failed ? tt.saveError : saving ? tt.saving : saved ? tt.saved : tt.autosave}
            </span>
          </section>

          <div {...stylex.props(channelStyles.settingsCol)}>
            <section {...stylex.props(channelStyles.card, channelStyles.cardRoom)} aria-label={tt.writingTitle}>
              <h2 {...stylex.props(channelStyles.cardHeading)}><Orb of="director" size={20} />{tt.writingTitle}</h2>
              <div {...stylex.props(channelStyles.form, channelStyles.formRoom)}>
                <div {...stylex.props(channelStyles.field)}>
                  <span {...stylex.props(channelStyles.fieldLabel)}>{tt.language}</span>
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
                  <p {...stylex.props(channelStyles.fieldHint, channelStyles.hintRoom)}>{tt.languageHint}</p>
                </div>

                <div {...stylex.props(channelStyles.field)}>
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

                <div {...stylex.props(channelStyles.field)}>
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
                  {footer.error ? <FieldError>{footer.error}</FieldError> : <p {...stylex.props(channelStyles.fieldHint, channelStyles.hintRoom)}>{tt.footerBadge} · {tt.footerHint}</p>}
                </div>

                <div {...stylex.props(channelStyles.field)}>
                  <span {...stylex.props(channelStyles.fieldRow)}>
                    <span {...stylex.props(channelStyles.fieldLabel)}>{tt.tags} <span {...stylex.props(channelStyles.fieldLabelNote)}>{tt.tagsBadge}</span></span>
                    <span {...stylex.props(channelStyles.counter, channelStyles.counterRoom)}>{tt.tagsCount(tags.length, 10)}</span>
                  </span>
                  <div {...stylex.props(channelStyles.tags)}>
                    {tags.map((tag) => (
                      <span {...stylex.props(channelStyles.tag)} key={tag}>
                        {tag}
                        <button {...stylex.props(channelStyles.tagButton)}
                          type="button"
                          aria-label={tt.removeTag(tag)}
                          onClick={() => void saveTags(tags.filter((item) => item !== tag))}
                        >
                          <Icon name="close" size={12} />
                        </button>
                      </span>
                    ))}
                    <input
                      {...stylex.props(channelStyles.tagPill)}
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
                  {tagError && <FieldError>{tagError}</FieldError>}
                </div>

                <p {...stylex.props(channelStyles.fieldHint, channelStyles.hintRoom)}>{tt.writingHint}</p>

                <LearnedRulesList scope={{ channelId: channel.id }} />
              </div>
            </section>

            <nav {...stylex.props(channelStyles.navrows)} aria-label={tt.navLabel}>
              <a {...stylex.props(channelStyles.navrow)} href={`#${paths.channel(channel.id, 'sources')}`}>
                <Icon name="radar" size={16} xstyle={channelStyles.navrowIc} />
                <span {...stylex.props(channelStyles.navrowText)}>
                  <span>{tt.linkSources}</span>
                  <span {...stylex.props(channelStyles.navrowSub)}>{tt.linkSourcesSub(channel.sources.length, channel.pacePerDay)}</span>
                </span>
                <Icon name="arrow-r" size={14} xstyle={channelStyles.navrowIc} />
              </a>
              <a {...stylex.props(channelStyles.navrow)} href={`#${paths.channel(channel.id, 'templates')}`}>
                <Icon name="layer" size={16} xstyle={channelStyles.navrowIc} />
                <span {...stylex.props(channelStyles.navrowText)}>
                  <span>{tt.linkTemplates}</span>
                  <span {...stylex.props(channelStyles.navrowSub)}>{tt.linkTemplatesSub(channel.templates.length)}</span>
                </span>
                <Icon name="arrow-r" size={14} xstyle={channelStyles.navrowIc} />
              </a>
              <a {...stylex.props(channelStyles.navrow)} href={`#${paths.settings('agents')}`}>
                <Icon name="magic" size={16} xstyle={channelStyles.navrowIc} />
                <span {...stylex.props(channelStyles.navrowText)}>
                  <span>{tt.linkAssistant}</span>
                  <span {...stylex.props(channelStyles.navrowSub)}>
                    {directorModel ? tt.linkAssistantModels(directorModel, drawLabel) : tt.linkAssistantSub}
                  </span>
                </span>
                <Icon name="arrow-r" size={14} xstyle={channelStyles.navrowIc} />
              </a>
            </nav>
          </div>
        </div>

        <section {...stylex.props(channelStyles.delete)} aria-label={tt.deleteTitle}>
          <div {...stylex.props(channelStyles.deleteText)}>
            <h2 {...stylex.props(channelStyles.deleteTitle)}>{tt.deleteTitle}</h2>
            <p {...stylex.props(channelStyles.deleteBody)}>{tt.deleteBody}</p>
          </div>
          <div {...stylex.props(channelStyles.deleteActions)}>
            <ActionButton xstyle={channelStyles.deleteBtn} variant="danger-soft" onClick={onDelete} icon={<Icon name="trash" size={15} />}>
              {tt.deleteButton}
            </ActionButton>
          </div>
        </section>
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
      <span {...stylex.props(channelStyles.fieldRow)}>
        <label {...stylex.props(channelStyles.fieldLabel)} htmlFor={htmlFor}>{label}</label>
        {right}
      </span>
      {children}
    </>
  )
}
