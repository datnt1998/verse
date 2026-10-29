import { expect, it } from 'vitest'
import { fieldValueLabel, toolEventText } from './labels'
import { toolArgRows } from '../screens/activity/mobile/logic'
import { nodesFromArgs, toolSummary } from './roundSummary'

it('chapter defaults and impossible chapter values never become visible parameter rows on any tool', () => {
  for (const chapter of [0, -1, 2147483647, 2147483648]) {
    expect(fieldValueLabel('chapter', String(chapter))).toBeNull()
    expect(toolArgRows({ args: JSON.stringify({ chapter }) })).toEqual([])
    expect(toolArgRows({ args: JSON.stringify({ from: chapter, to: chapter }) })).toEqual([])
    expect(toolEventText({ summary: 'novel_context', args: JSON.stringify({ chapter }), agent: 'architect' })).toBe('Đọc bối cảnh')
    expect(toolSummary('novel_context', nodesFromArgs(JSON.stringify({ chapter })))).not.toMatch(/chương|214748364/)
  }
  expect(fieldValueLabel('chapter', '40')).toBe('40')
  expect(toolArgRows({ args: '{"chapter":40}' })).toEqual([{ label: 'Chương', value: '40' }])
  expect(toolArgRows({ chapter: 41 })).toEqual([])
})
