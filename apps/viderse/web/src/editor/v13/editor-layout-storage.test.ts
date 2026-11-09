import { describe, expect, it } from 'vitest'
import { createLayoutStorage } from './editor-layout-storage'

const ids = ['library', 'preview', 'inspector']
const key = 'react-resizable-panels:viderse-v13-p-horizontal-v1'

describe('editor layout storage boundary', () => {
  it.each([
    'not json', 'null', '[]', 'true', '{}',
    '{"library":25,"preview":50}',
    '{"library":25,"preview":50,"other":25}',
    '{"library":"25","preview":50,"inspector":25}',
    '{"library":-25,"preview":100,"inspector":25}',
    '{"library":0,"preview":75,"inspector":25}',
    '{"library":25,"preview":25,"inspector":25}',
    '{"library":1e999,"preview":50,"inspector":25}',
  ])('rejects malformed or incompatible preferences: %s', raw => {
    const storage = createLayoutStorage(ids, () => ({ getItem: () => raw, setItem() {} }))
    expect(storage.getItem(key)).toBeNull()
  })

  it('ignores saved preferences when browser storage is inaccessible', () => {
    const storage = createLayoutStorage(ids, () => { throw new Error('SecurityError') })
    expect(storage.getItem(key)).toBeNull()
  })

})
