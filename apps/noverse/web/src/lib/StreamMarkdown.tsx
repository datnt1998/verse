import { memo } from 'react'
import { humanizeFreeText } from './labels'
import { ChapterContent, parseChapterText } from './markdown'

/** Shared by desktop cards and the mobile stream; raw identifiers stay in technical details. */
export const StreamMarkdown = memo(function StreamMarkdown({ text }: { text: string }) {
  if (text === '') return null
  return <ChapterContent blocks={parseChapterText(humanizeFreeText(text))} unmatchedRevisions={[]} showDiff={false} />
})
