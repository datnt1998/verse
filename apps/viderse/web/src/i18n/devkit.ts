// Chữ của DevKit (chỉ có ở bản dev): chữ khung app + khối `devkit`.
import { base } from './vi'
import { devkitCopy } from './devkitCopy'

export const t = { ...base, devkit: devkitCopy.devkit } as const
