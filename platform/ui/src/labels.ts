// Chuỗi giao diện của kit — gói mặc định tiếng Anh; app nạp tiếng Việt một lần
// qua setKitLabels(KIT_LABELS_VI) (labels.vi.ts) trước khi render. Đọc tại render
// nên mọi component tự nhận chuỗi mới, không cần context/provider.

export type KitLabels = {
  /** nhãn chung */
  cancel: string
  retry: string
  close: string
  /** Dialog */
  dialog: { close: string }
  /** toast: nhãn nút đóng + vùng thông báo */
  toast: {
    notifications: string
    dismiss: string
    actionFailed: string
    autoDismissAfter: (sec: number) => string
    pausedAutoDismiss: string
    /** toast không hẹn giờ: ở lại tới khi bạn đóng */
    staysUntilDismissed: string
  }
  /** controls */
  controls: { busyChanging: string; done: string }
  inputs: {
    clear: string
    decrease: string
    increase: string
    value: string
    rangeHint: (min: number, max: number) => string
    editLabel: string
    showPassword: string
    hidePassword: string
  }
  /** Selects (KitSelect/RichSelect/Combobox/MultiCombobox) */
  selects: {
    /** danh từ mặc định trong thông điệp (caller truyền noun= để đổi) */
    noun: string
    choose: string
    chooseItem: string
    chooseMany: string
    options: string
    empty: string
    loadingList: (noun: string) => string
    search: string
    searchMany: string
    clearSearch: string
    openList: string
    closeList: string
    loadFailed: (noun: string) => string
    noMatch: (noun: string, query: string) => string
    noMatchHint: (noun: string) => string
    noMatchGeneric: string
    addProvider: string
    removeTag: (label: string) => string
    limitReached: (limit: number) => string
    previewStart: (label: string) => string
    previewStop: (label: string) => string
    create: (text: string) => string
    manageVoices: string
    currentLabel: string
  }
  /** ConfirmDialog */
  confirm: {
    typeToConfirmPre: string
    typeToConfirmPost: string
    actionFailed: string
    dontAskAgain: string
    holdToConfirm: string
    holdMore: (sec: string) => string
    typedHintOk: string
    typedHintIdle: string
    undo: string
    retry: string
    pendingGeneric: string
  }
  /** Menus / Overlays */
  menus: { actions: string }
  overlays: { info: string }
  /** SlowLoadingNotice */
  slow: { notice: string }
  /** StaleBanner */
  stale: { loadFailed: string; retry: string }
  /** ConflictBar */
  conflict: { changedElsewhere: string; viewNew: string; overwrite: string }
  /** ScrollTopButton */
  scrollTop: { label: string; hint: string; read: (percent: number) => string }
  /** BrandLockup */
  brand: { name: string; studio: string }
  /** QrCode */
  qr: {
    codeLabel: string
    expired: string
    renew: string
    zoomLabel: string
    zoom: string
    zoomTitle: string
    zoomHint: string
  }
  /** Orbs */
  orbs: { thinking: string }
  /** Sheet (kéo để đóng) */
  sheet: { grabHandle: string; defaultLabel: string }
}

const DEFAULTS: KitLabels = {
  cancel: 'Cancel',
  retry: 'Retry',
  close: 'Close',
  dialog: { close: 'Close' },
  toast: {
    notifications: 'Notifications',
    dismiss: 'Dismiss notification',
    actionFailed: 'Action failed',
    autoDismissAfter: (sec) => `Auto-dismiss in ${sec} seconds`,
    pausedAutoDismiss: 'Auto-dismiss paused',
    staysUntilDismissed: 'Stays until you dismiss it',
  },
  controls: { busyChanging: 'Updating', done: 'Done' },
  inputs: {
    clear: 'Clear',
    decrease: 'Decrease',
    increase: 'Increase',
    value: 'Value',
    rangeHint: (min, max) => `From ${min} to ${max}.`,
    editLabel: 'Edit in Settings',
    showPassword: 'Show password',
    hidePassword: 'Hide password',
  },
  selects: {
    noun: 'option',
    choose: 'Choose…',
    chooseItem: 'Select an item',
    chooseMany: 'Select multiple items',
    options: 'Options',
    empty: 'No options yet.',
    loadingList: (noun) => `Loading ${noun}…`,
    search: 'Search options…',
    searchMany: 'Select or type to add…',
    clearSearch: 'Clear search',
    openList: 'Open list',
    closeList: 'Close list',
    loadFailed: (noun) => `Could not load ${noun} list`,
    noMatch: (noun, query) => `No ${noun} matches “${query}”`,
    noMatchHint: (noun) => `The ${noun} you need is not there? Add another provider.`,
    noMatchGeneric: 'No matching option.',
    addProvider: 'Add provider…',
    removeTag: (label) => `Remove tag ${label}`,
    limitReached: (limit) => `Selected up to ${limit} items. Remove one to add more.`,
    previewStart: (label) => `Preview ${label}`,
    previewStop: (label) => `Stop preview ${label}`,
    create: (text) => `Add “${text}”`,
    manageVoices: 'Manage voices…',
    currentLabel: 'Selected: ',
  },
  confirm: {
    typeToConfirmPre: 'Type',
    typeToConfirmPost: 'to confirm',
    actionFailed: 'Could not complete. Please try again.',
    dontAskAgain: 'Do not ask again',
    holdToConfirm: 'Hold to confirm',
    holdMore: (sec) => `Keep holding… ${sec}s`,
    typedHintOk: 'Match — safe to proceed.',
    typedHintIdle: 'The button stays off until the phrase matches. Outer spaces do not matter.',
    undo: 'Undo',
    retry: 'Retry',
    pendingGeneric: 'Working…',
  },
  menus: { actions: 'More actions' },
  overlays: { info: 'Info' },
  slow: { notice: 'Slower than usual…' },
  stale: { loadFailed: 'Could not reload', retry: 'Retry' },
  conflict: { changedElsewhere: 'Changed elsewhere', viewNew: 'View new version', overwrite: 'Overwrite' },
  scrollTop: { label: 'Back to top', hint: 'Scroll to top', read: (percent) => `read ${percent}%` },
  brand: { name: 'Noverse', studio: 'Studio' },
  qr: {
    codeLabel: 'Login QR code',
    expired: 'Expired',
    renew: 'Renew',
    zoomLabel: 'Enlarge QR code',
    zoom: 'Enlarge',
    zoomTitle: 'QR code',
    zoomHint: 'Scan with your phone to continue.',
  },
  orbs: { thinking: 'Writer is writing' },
  sheet: { grabHandle: 'Drag down to close', defaultLabel: 'Options' },
}

let current: KitLabels = DEFAULTS

export const getKitLabels = (): KitLabels => current

/** Nạp nhãn của app (merge nông theo nhóm — app truyền đủ một nhóm thì nhóm đó thay hẳn). */
export function setKitLabels(patch: Partial<KitLabels>): void {
  current = { ...current, ...patch }
}
