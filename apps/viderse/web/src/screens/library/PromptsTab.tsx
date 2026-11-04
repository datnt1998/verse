import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Tab Prompt ảnh (Library.dc.html › tabs=prompts): lọc loại + đếm, tìm theo
// tên/nội dung (máy chủ lọc q, debounce 250 ms), hai cụm Ảnh cảnh / Ảnh bìa
// (mỗi cụm kết thúc bằng tile "Prompt mới"), prompt có sẵn trước prompt của
// bạn, rồi khung sửa. Tìm/lọc một loại thì vẽ một lưới duy nhất (có ảo hoá).
import { useEffect, useRef, useState } from 'react'
import { Icon, TextField, toast } from '@noverse/ui'
import { Segmented } from '../../components/Segmented'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { useDebounced } from '../../lib/useDebounced'
import { useVirtualGrid } from '../../lib/useVirtualGrid'
import { createPrompt, listPrompts, type ImagePrompt, type PromptKind } from '../../api/library'
import type { PromptView } from '../../router'
import { PromptEditor } from './PromptEditor'
import { num } from './fmt'

type Filter = 'all' | PromptKind

const EMPTY: ImagePrompt[] = []

const ordered = (items: ImagePrompt[]) => [...items]
  .sort((a, b) => Number(b.builtIn) - Number(a.builtIn) || a.name.localeCompare(b.name, 'vi'))

function PromptCard({ prompt, onOpen, onDuplicate }: {
  prompt: ImagePrompt
  onOpen: () => void
  onDuplicate: () => void
}) {
  const copy = t.library.prompts
  const names = prompt.usedBy.map((ref) => ref.name)
  // Ảnh mẫu do máy chủ dựng sẵn; hỏng/404 thì quay về ô trung tính, không bịa ảnh khác.
  // Khoá theo URL: ảnh mẫu mới thay ảnh hỏng thì vẫn hiện.
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const sampleUrl = prompt.sampleUrls?.[0]
  const showArt = sampleUrl !== undefined && sampleUrl !== '' && sampleUrl !== failedUrl
  return (
    <article className={`vlb-pcard ${stylex.props(sx.pcard).className}`}>
      <button type="button" className={`vlb-pcard-open ${stylex.props(sx.pcardOpen).className}`} onClick={onOpen} aria-label={copy.openAria(prompt.name)}>
        <span className={`vlb-pcard-art ${stylex.props(sx.pcardArt, ((prompt.kind) === "cover") && sx.pcardArtDataKindCover).className}`} data-kind={prompt.kind} data-comp={prompt.composition ?? 'clean'}>
          {showArt && (
            <img className={`vlb-pcard-img ${stylex.props(sx.pcardImg).className}`} src={sampleUrl} alt="" loading="lazy"
              onError={() => setFailedUrl(sampleUrl ?? null)} />
          )}
          {showArt && <span className={`vlb-pcard-veil ${stylex.props(sx.pcardVeil).className}`} aria-hidden="true" />}
          {!showArt && <span className={`vlb-pcard-glyph ${stylex.props(sx.pcardGlyph).className}`} aria-hidden="true"><Icon name={prompt.kind === 'cover' ? 'magicpen' : 'gallery'} size={22} /></span>}
          <span className={`vlb-pcard-pill ${stylex.props(sx.pcardPill).className}`}>{copy.kindName[prompt.kind]}</span>
          <span className={`vlb-pcard-name ${stylex.props(sx.pcardName).className}`}>{prompt.name}</span>
        </span>
      </button>
      <div className={`vlb-pcard-body ${stylex.props(sx.pcardBody).className}`}>
        <p className={`vlb-pcard-text ${stylex.props(sx.pcardText).className}`}>{prompt.prompt}</p>
        <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>
          {prompt.builtIn ? copy.editedBuilt : copy.usedText(names)}
        </p>
        <div className={`vlb-pcard-foot ${stylex.props(sx.pcardFoot).className}`}>
          {prompt.builtIn && <span className={`vlb-chip vlb-chip--acc ${stylex.props(sx.chipChipAcc).className}`}>{copy.builtChip}</span>}
          <span className={`vlb-pcard-actions ${stylex.props(sx.pcardActions).className}`}>
            <button type="button" className={`vlb-linkbtn ${stylex.props(sx.linkbtn).className}`} onClick={onOpen}>{t.library.templates.open}</button>
            <button type="button" className={`vlb-iconbtn ${stylex.props(sx.iconbtn).className}`} aria-label={copy.duplicateAria(prompt.name)} title={copy.duplicateAria(prompt.name)} onClick={onDuplicate}>
              <Icon name="copy" size={15} />
            </button>
          </span>
        </div>
      </div>
    </article>
  )
}

function NewTile({ onClick }: { onClick: () => void }) {
  const copy = t.library.prompts
  return (
    <button type="button" className={`vlb-newtile ${stylex.props(sx.newtile).className}`} onClick={onClick}>
      <span className={`vlb-newtile-icon ${stylex.props(sx.newtileIcon).className}`} aria-hidden="true"><Icon name="add" size={18} /></span>
      <span className={`vlb-newtile-title ${stylex.props(sx.newtileTitle).className}`}>{copy.newTileTitle}</span>
      <span className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.newTileHint}</span>
    </button>
  )
}

export function PromptsTab({ view, all, allLoading, onView }: {
  view: PromptView | undefined
  all: ImagePrompt[]
  /** true khi danh sách toàn cục chưa tải xong — không được chọn prompt mở */
  allLoading: boolean
  onView: (view: PromptView | undefined) => void
}) {
  const copy = t.library.prompts
  const [filter, setFilter] = useState<Filter>('all')
  const [query, setQuery] = useState('')
  const debounced = useDebounced(query)
  const list = useResource(
    keys.prompts(filter === 'all' ? '' : filter, debounced),
    ({ signal }) => listPrompts({ ...(filter === 'all' ? {} : { kind: filter }), ...(debounced ? { q: debounced } : {}) }, signal),
  )
  const [editing, setEditing] = useState<{ mode: 'new' } | { mode: 'edit'; prompt: ImagePrompt } | null>(null)
  const lastView = useRef<PromptView | undefined>(undefined)

  useEffect(() => {
    if (view === lastView.current) return
    if ((view === 'mine' || view === 'builtin') && allLoading) return
    lastView.current = view
    if (!view || view === 'list') { setEditing(null); return }
    if (view === 'new') { setEditing({ mode: 'new' }); return }
    const pick = all.find((prompt) => (view === 'builtin' ? prompt.builtIn : !prompt.builtIn))
    setEditing(pick ? { mode: 'edit', prompt: pick } : { mode: 'new' })
  }, [view, allLoading, all])

  const closeEditor = () => { setEditing(null); onView(undefined) }

  const items = ordered(list.data?.items ?? EMPTY)
  const sceneCount = all.filter((prompt) => prompt.kind === 'scene').length
  const coverCount = all.filter((prompt) => prompt.kind === 'cover').length
  const grouped = filter === 'all' && !debounced
  const cells = items.length + 1
  const grid = useVirtualGrid({ count: cells, metrics: { cardMinW: 248, cardExtraH: 168, gapX: 14, gapY: 16 } })

  const duplicate = async (prompt: ImagePrompt) => {
    try {
      const copyOf = await createPrompt({ name: copy.copyName(prompt.name), kind: prompt.kind, prompt: prompt.prompt, fromId: prompt.id })
      await Promise.all([invalidate(keys.prompts('', '')), invalidate(keys.prompts('scene', '')), invalidate(keys.prompts('cover', ''))])
      setEditing({ mode: 'edit', prompt: copyOf })
      toast(copy.editor.duplicated, 'ok')
    } catch (err) {
      toast(err instanceof Error ? err.message : copy.editor.duplicateFail, 'err')
    }
  }

  const openNew = () => setEditing({ mode: 'new' })

  if (editing) {
    return (
      <PromptEditor
        key={editing.mode === 'new' ? 'new' : editing.prompt.id}
        prompt={editing.mode === 'new' ? null : editing.prompt}
        onClose={closeEditor}
        onSaved={(saved) => { setEditing({ mode: 'edit', prompt: saved }); onView(saved.builtIn ? 'builtin' : 'mine') }}
        onDeleted={closeEditor}
        onDuplicate={editing.mode === 'edit' ? () => void duplicate(editing.prompt) : undefined}
      />
    )
  }

  const renderGrid = (subset: ImagePrompt[], groupEnd = true) => (
    <div className={`vlb-grid vlb-grid--prompts ${stylex.props(sx.gridGridPrompts).className}`}>
      {subset.map((prompt) => (
        <PromptCard key={prompt.id} prompt={prompt}
          onOpen={() => setEditing({ mode: 'edit', prompt })}
          onDuplicate={() => void duplicate(prompt)} />
      ))}
      {groupEnd && <NewTile onClick={openNew} />}
    </div>
  )

  return (
    <div className={`vlb-scroll ${stylex.props(sx.scroll).className}`} ref={grid.ref} onScroll={grid.onScroll}>
      <p className={`vlb-intro ${stylex.props(sx.intro).className}`}>{copy.intro}</p>
      <div className={`vlb-toolbar ${stylex.props(sx.toolbar).className}`}>
        <Segmented buttonStyle={sx.touchTarget}
          label={copy.filterAria}
         
          value={filter}
          onChange={(id) => setFilter(id as Filter)}
          options={[
            { id: 'all', label: `${copy.filterAll} ${all.length}` },
            { id: 'scene', label: `${copy.filterScene} ${sceneCount}` },
            { id: 'cover', label: `${copy.filterCover} ${coverCount}` },
          ]}
        />
        <div className={`vlb-search ${stylex.props(sx.search).className}`}>
          <Icon name="search" size={15} />
          <TextField inputStyle={[sx.vlbSearchTextField]} aria-label={copy.searchLabel} placeholder={copy.searchPlaceholder}
            value={query} onChange={setQuery} />
        </div>
      </div>

      {items.length === 0 && !list.initialLoading && (
        <p className={`vlb-empty-title ${stylex.props(sx.emptyTitle).className}`}>{copy.emptyFiltered}</p>
      )}

      {grouped ? (
        <>
          <div className={`vlb-grp ${stylex.props(sx.grp).className}`}>
            <span className={`vlb-grp-title ${stylex.props(sx.grpTitle).className}`}>{copy.scenesTitle} {num(all.filter((p) => p.kind === 'scene').length)}</span>
            <span className={`vlb-grp-sep ${stylex.props(sx.grpSep).className}`}>·</span>
            <span>{copy.scenesHelp}</span>
          </div>
          {renderGrid(ordered(items.filter((p) => p.kind === 'scene')))}
          <div className={`vlb-grp ${stylex.props(sx.grp).className}`}>
            <span className={`vlb-grp-title ${stylex.props(sx.grpTitle).className}`}>{copy.coversTitle} {num(all.filter((p) => p.kind === 'cover').length)}</span>
            <span className={`vlb-grp-sep ${stylex.props(sx.grpSep).className}`}>·</span>
            <span>{copy.coversHelp}</span>
          </div>
          {renderGrid(ordered(items.filter((p) => p.kind === 'cover')))}
        </>
      ) : (
        <>
          <div className={`vlb-legend ${stylex.props(sx.legend).className}`}>
            {(filter === 'all' || filter === 'scene') && (
              <span><strong {...stylex.props(sx.vlbLegendStrong)}>{copy.scenesTitle}</strong> {num(sceneCount)} · {copy.scenesHelp}</span>
            )}
            {(filter === 'all' || filter === 'cover') && (
              <span><strong {...stylex.props(sx.vlbLegendStrong)}>{copy.coversTitle}</strong> {num(coverCount)} · {copy.coversHelp}</span>
            )}
          </div>
          <div className={`vlb-grid vlb-grid--prompts ${stylex.props(sx.gridGridPrompts).className}`} style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
            {items.slice(grid.itemStart, grid.itemEnd).map((prompt) => (
              <PromptCard key={prompt.id} prompt={prompt}
                onOpen={() => setEditing({ mode: 'edit', prompt })}
                onDuplicate={() => void duplicate(prompt)} />
            ))}
            {grid.itemEnd >= cells && <NewTile onClick={openNew} />}
          </div>
        </>
      )}
    </div>
  )
}

const sx = stylex.create({
  pcard: { display: "flex", flexDirection: "column", minWidth: 0, backgroundColor: color["--color-card"], backgroundImage: "none", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", overflowX: "hidden", overflowY: "hidden" },
  pcardOpen: { display: "block", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", cursor: "pointer", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  pcardArt: { display: "block", position: "relative", aspectRatio: libraryVars.videoAspectRatio, backgroundColor: "transparent", backgroundImage: libraryVars.promptsTabPcardArtBackgroundImage, overflowX: "hidden", overflowY: "hidden" },
  pcardArtDataKindCover: { backgroundColor: "transparent", backgroundImage: libraryVars.promptsTabPcardArtDataKindCoverBackgroundImage },
  pcardImg: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  pcardVeil: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, backgroundColor: "transparent", backgroundImage: libraryVars.overlaysTabOartVeilBackgroundImage },
  pcardGlyph: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, display: "flex", alignItems: "center", justifyContent: "center", color: color["--color-fg-4"], opacity: libraryVars.promptsTabPcardGlyphOpacity },
  pcardPill: { position: "absolute", top: "var(--vv-s-8)", left: "var(--vv-s-8)", paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", backgroundColor: "var(--vv-glass)", backgroundImage: "none", backdropFilter: libraryVars.overlaysTabOartPlayBackdropFilter, fontSize: libraryVars.microFontSize, color: color["--color-fg-2"] },
  pcardName: { position: "absolute", left: "var(--vv-s-10)", right: "var(--vv-s-10)", bottom: "var(--vv-s-9)", fontSize: libraryVars.controlFontSize, fontWeight: libraryVars.labelWeight, color: color["--color-fg"], overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", textAlign: "left" },
  pcardBody: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-10)", paddingRight: "var(--vv-s-11)", paddingBottom: "var(--vv-s-11)", paddingLeft: "var(--vv-s-11)", minWidth: 0 },
  pcardText: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-3"], lineHeight: libraryVars.bodyLineHeight, display: "-webkit-box", WebkitLineClamp: libraryVars.promptsTabPcardTextWebkitLineClamp, WebkitBoxOrient: "vertical", overflowX: "hidden", overflowY: "hidden", overflowWrap: "anywhere" },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  pcardFoot: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  chipChipAcc: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "var(--vv-accent-45)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "var(--vv-accent-45)", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "var(--vv-accent-45)", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "var(--vv-accent-45)", color: color["--color-accent"], fontSize: "var(--vv-s-11)", whiteSpace: "nowrap" },
  pcardActions: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", marginLeft: "auto" },
  linkbtn: { paddingTop: "var(--vv-s-3)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-3)", paddingLeft: "var(--vv-s-6)", borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: "none", ":hover": "none" }, color: { default: color["--color-fg-2"], ":hover": color["--color-fg"] }, fontFamily: "inherit", fontSize: "var(--vv-s-12)", fontStyle: "inherit", fontWeight: "inherit", fontStretch: "inherit", fontVariant: "inherit", lineHeight: "inherit", cursor: "pointer", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": "var(--vv-s-1)" } },
  iconbtn: { display: "inline-flex", alignItems: "center", justifyContent: "center", width: "var(--vv-s-26)", height: "var(--vv-s-26)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "transparent", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "transparent", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "transparent", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "transparent", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover)" }, backgroundImage: { default: "none", ":hover": "none" }, color: { default: color["--color-fg-4"], ":hover": color["--color-fg"] }, cursor: "pointer", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, minWidth: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": "var(--vv-s-1)" } },
  newtile: { minHeight: { default: libraryVars.overlaysTabNewtileNewtileDropMinHeight, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, display: "flex", flexDirection: "column", alignItems: "flex-start", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "dashed", borderTopColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "dashed", borderRightColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "dashed", borderBottomColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "dashed", borderLeftColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: { default: "transparent", ":hover": "var(--vv-accent-6)" }, backgroundImage: { default: "none", ":hover": "none" }, color: color["--color-fg-2"], cursor: "pointer", textAlign: "left" },
  newtileIcon: { display: "inline-flex", alignItems: "center", justifyContent: "center", width: "var(--vv-s-30)", height: "var(--vv-s-30)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-3"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-3"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-3"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-3"], color: color["--color-fg-3"] },
  newtileTitle: { fontSize: libraryVars.controlFontSize, fontWeight: libraryVars.labelWeight },
  gridGridPrompts: { display: "grid", rowGap: { default: "var(--vv-s-14)", "@media (max-width: 767px)": "var(--vv-s-12)" }, columnGap: { default: "var(--vv-s-14)", "@media (max-width: 767px)": "var(--vv-s-12)" }, gridTemplateColumns: { default: libraryVars.promptsTabGridGridPromptsGridTemplateColumns2, "@media (max-width: 767px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, alignContent: "start" },
  scroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, overflowY: "auto", overscrollBehavior: "contain", paddingTop: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-4)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-24)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)" },
  intro: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, maxWidth: libraryVars.promptsTabIntroMaxWidth, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight },
  toolbar: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  search: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.promptsTabSearchFlexBasis, minWidth: 0, color: color["--color-fg-4"], minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  vlbSearchTextField: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minWidth: 0, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  emptyTitle: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg-2"] },
  grp: { display: "flex", flexWrap: "wrap", alignItems: "baseline", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", marginTop: "var(--vv-s-16)", marginRight: 0, marginBottom: "var(--vv-s-8)", marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-4"] },
  grpTitle: { color: color["--color-fg-2"], fontWeight: libraryVars.labelWeight },
  grpSep: { color: color["--color-fg-4"] },
  legend: { display: "flex", flexWrap: "wrap", rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-16)", fontSize: "var(--vv-s-12)", color: color["--color-fg-3"] },
  vlbLegendStrong: { color: color["--color-fg-2"], fontWeight: libraryVars.labelWeight },
})
