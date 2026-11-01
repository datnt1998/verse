// Ví dụ trước → sau của một quy tắc/đề xuất học, hiện ngay dưới hàng (không mở hộp thoại).
import * as stylex from '@stylexjs/stylex'
import { t } from '../../i18n/vi'
import { learnStyles as s } from './learn.stylex'

const c = t.learn.learn

export function RuleExamples({ examples }: { examples: { before: string; after: string }[] }) {
  return <ul {...stylex.props(s.examples)}>
    {examples.map((example, index) => <li key={index} {...stylex.props(s.example)}>
      <span {...stylex.props(s.exampleRow)}><span {...stylex.props(s.exampleTag)}>{c.before}</span><span>{example.before}</span></span>
      <span {...stylex.props(s.exampleRow)}><span {...stylex.props(s.exampleTag)}>{c.after}</span><span>{example.after}</span></span>
    </li>)}
  </ul>
}
