// @vitest-environment node

import { describe, expect, it } from 'vitest'
import * as keyframeEditors from './keyframe-editors'

describe('keyframe editor adapter', () => {
  it('loads the keyframe logic exports timeline actions rely on', () => {
    expect(keyframeEditors.getEffectPropertyBaseValue).toBeTypeOf('function')
    expect(keyframeEditors.getAnimatablePropertiesForItem).toBeTypeOf('function')
    expect(keyframeEditors.resolveAnimatedShapeItem).toBeTypeOf('function')
  })
})
