import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { exploreStyles } from './explore.stylex'
// Ô "Tìm tiêu đề" của Khám phá (artboard Explore · search=none|match|nomatch):
// 240px ngay trước "Chọn nhiều". Phím "/" (ngoài ô nhập khác) đưa focus vào ô;
// Esc trong ô xoá chữ. Có chữ → nút ✕ cuối ô; trống → gợi ý phím "/".
import { useEffect, useId, useRef } from 'react'
import { Icon } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'

const isTyping = (target: EventTarget | null) =>
  target instanceof HTMLElement && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))

export function SearchBox({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const tt = t.screens.explore
  const id = useId()
  const input = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return
      event.preventDefault()
      input.current?.focus()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div role="search" className={`vx-search ${stylex.props(exploreStyles.search).className}`}>
      <label htmlFor={id} {...stylex.props(text.srOnly)}>{tt.searchLabel}</label>
      <Icon name="search" size={14} className={`vx-search__ic ${stylex.props(exploreStyles.searchIc).className}`} aria-hidden />
      <input {...stylex.props(exploreStyles.searchInput)}
        id={id}
        ref={input}
        type="text"
        enterKeyHint="search"
        autoComplete="off"
        spellCheck={false}
        maxLength={200}
        placeholder={tt.searchPlaceholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key !== 'Escape' || !value) return
          e.preventDefault()
          e.stopPropagation()
          onChange('')
        }}
      />
      {value ? (
        <button type="button" className={`vx-search__clear ${stylex.props(exploreStyles.searchClear).className}`} aria-label={tt.searchClear}
          onClick={() => { onChange(''); input.current?.focus() }}>
          <Icon name="close" size={13} />
        </button>
      ) : (
        <span className={`vx-search__kbd ${stylex.props(exploreStyles.searchKbd).className}`} aria-hidden="true">/</span>
      )}
    </div>
  )
}
