// Bản ĐẦY ĐỦ chữ của app — chỉ dùng cho test và công cụ đọc chữ một chỗ.
// App không import tệp này: mỗi màn nạp module chữ của chính nó (./vi cho khung
// app, ./library, ./channels, ./explore, ./queue, ./settings, ./learn, …) để
// chunk của màn kia không kéo chữ của màn này. Test src/i18n/all.test.ts canh
// điều đó.
import { accountsCopy } from './accounts'
import { aiCopy } from './ai'
import { authCopy } from './auth'
import { autonomyCopy } from './autonomy'
import { backdropCopy } from './backdrop'
import { channelsCopy } from './channels'
import { devkitCopy } from './devkitCopy'
import { errorsCopy } from './errorsCopy'
import { learnCopy } from './learn'
import { libraryCopy } from './library'
import { logs } from './logs'
import { screenEditor, screenExplore, screenNew, screenQueue, screenReview, screenSettings, screenWorkbench } from './screens'
import { settings } from './settings'
import { setup } from './setup'
import { storage } from './storage'
import { tools } from './tools'
import { base } from './vi'

export const t = {
  ...base,
  ...authCopy,
  screens: {
    workbench: screenWorkbench,
    review: screenReview,
    queue: screenQueue,
    explore: screenExplore,
    new: screenNew,
    editor: screenEditor,
    settings: screenSettings,
  },
  ai: aiCopy,
  accounts: accountsCopy,
  settings,
  tools,
  storage,
  logs,
  prep: setup,
  library: libraryCopy,
  backdrop: backdropCopy,
  channels: channelsCopy,
  autonomy: autonomyCopy,
  learn: learnCopy,
  errors: errorsCopy.errors,
  devkit: devkitCopy.devkit,
} as const

export type ViStrings = typeof t
