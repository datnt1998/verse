// Tab Prompt ảnh (Library.dc.html › tabs=prompts): lọc loại + đếm, tìm theo
// tên/nội dung (máy chủ lọc q, debounce 250 ms), hai cụm Ảnh cảnh / Ảnh bìa
// (mỗi cụm kết thúc bằng tile "Prompt mới"), prompt có sẵn trước prompt của
// bạn, rồi khung sửa. Tìm/lọc một loại thì vẽ một lưới duy nhất (có ảo hoá).
import { useEffect, useRef, useState } from 'react'
import { Icon, Segmented, TextField } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
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
  return (
    <article className="vlb-pcard">
      <button type="button" className="vlb-pcard-open" onClick={onOpen} aria-label={copy.openAria(prompt.name)}>
        <span className="vlb-pcard-art" data-kind={prompt.kind} data-comp={prompt.composition ?? 'clean'}>
          <span className="vlb-pcard-glyph" aria-hidden="true"><Icon name={prompt.kind === 'cover' ? 'magicpen' : 'gallery'} size={22} /></span>
          <span className="vlb-pcard-pill">{copy.kindName[prompt.kind]}</span>
          <span className="vlb-pcard-name">{prompt.name}</span>
        </span>
      </button>
      <div className="vlb-pcard-body">
        <p className="vlb-pcard-text">{prompt.prompt}</p>
        <p className="vlb-hint">
          {prompt.builtIn ? copy.editedBuilt : copy.usedText(names)}
        </p>
        <div className="vlb-pcard-foot">
          {prompt.builtIn && <span className="vlb-chip vlb-chip--acc">{copy.builtChip}</span>}
          <span className="vlb-pcard-actions">
            <button type="button" className="vlb-linkbtn" onClick={onOpen}>{t.library.templates.open}</button>
            <button type="button" className="vlb-iconbtn" aria-label={copy.duplicateAria(prompt.name)} title={copy.duplicateAria(prompt.name)} onClick={onDuplicate}>
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
    <button type="button" className="vlb-newtile" onClick={onClick}>
      <span className="vlb-newtile-icon" aria-hidden="true"><Icon name="add" size={18} /></span>
      <span className="vlb-newtile-title">{copy.newTileTitle}</span>
      <span className="vlb-hint">{copy.newTileHint}</span>
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
    if (view === lastView.current && !allLoading) return
    if (view === lastView.current) return
    lastView.current = view
    if (!view || view === 'list') { setEditing(null); return }
    if (view === 'new') { setEditing({ mode: 'new' }); return }
    // mine|builtin: chờ danh sách tải xong mới chọn, tránh rơi về "Prompt mới"
    if (allLoading) return
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
    } catch {
      // lỗi hiện trong khung sửa; ở đây bỏ qua để không chặn lưới
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
    <div className="vlb-grid vlb-grid--prompts">
      {subset.map((prompt) => (
        <PromptCard key={prompt.id} prompt={prompt}
          onOpen={() => setEditing({ mode: 'edit', prompt })}
          onDuplicate={() => void duplicate(prompt)} />
      ))}
      {groupEnd && <NewTile onClick={openNew} />}
    </div>
  )

  return (
    <div className="vlb-scroll" ref={grid.ref} onScroll={grid.onScroll}>
      <p className="vlb-intro">{copy.intro}</p>
      <div className="vlb-toolbar">
        <Segmented
          label={copy.filterAria}
          fit="content"
          value={filter}
          onChange={(id) => setFilter(id as Filter)}
          options={[
            { id: 'all', label: `${copy.filterAll} ${all.length}` },
            { id: 'scene', label: `${copy.filterScene} ${sceneCount}` },
            { id: 'cover', label: `${copy.filterCover} ${coverCount}` },
          ]}
        />
        <div className="vlb-search">
          <Icon name="search" size={15} />
          <TextField aria-label={copy.searchLabel} placeholder={copy.searchPlaceholder}
            value={query} onChange={setQuery} />
        </div>
      </div>

      {items.length === 0 && !list.initialLoading && (
        <p className="vlb-empty-title">{copy.emptyFiltered}</p>
      )}

      {grouped ? (
        <>
          <div className="vlb-grp">
            <span className="vlb-grp-title">{copy.scenesTitle} {num(all.filter((p) => p.kind === 'scene').length)}</span>
            <span className="vlb-grp-sep">·</span>
            <span>{copy.scenesHelp}</span>
          </div>
          {renderGrid(ordered(items.filter((p) => p.kind === 'scene')))}
          <div className="vlb-grp">
            <span className="vlb-grp-title">{copy.coversTitle} {num(all.filter((p) => p.kind === 'cover').length)}</span>
            <span className="vlb-grp-sep">·</span>
            <span>{copy.coversHelp}</span>
          </div>
          {renderGrid(ordered(items.filter((p) => p.kind === 'cover')))}
        </>
      ) : (
        <>
          <div className="vlb-legend">
            {(filter === 'all' || filter === 'scene') && (
              <span><strong>{copy.scenesTitle}</strong> {num(sceneCount)} · {copy.scenesHelp}</span>
            )}
            {(filter === 'all' || filter === 'cover') && (
              <span><strong>{copy.coversTitle}</strong> {num(coverCount)} · {copy.coversHelp}</span>
            )}
          </div>
          <div className="vlb-grid vlb-grid--prompts" style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
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
