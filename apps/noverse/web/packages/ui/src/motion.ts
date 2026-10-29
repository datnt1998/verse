// Motion presets dùng chung của kit — easing/thời lượng trích từ artboard (17-kit, 03-tokens).
// Component stylex vẫn khai báo animation của riêng nó; đây là tập hằng số để khớp nhịp.
export type MotionPreset = {
  /** cubic-bezier(.2,.9,.3,1) — popover/tooltip của Kit */
  readonly popIn: string
  /** cubic-bezier(.2,.9,.25,1.12) — toast/sheet vào */
  readonly springIn: string
  /** cubic-bezier(.5,0,.75,0) — đóng sheet/dialog */
  readonly exit: string
  /** cubic-bezier(.2,.8,.2,1) — ease mặc định (--ease-out) */
  readonly easeOut: string
  /** cubic-bezier(.2,.9,.25,1) — thanh tab mobile */
  readonly slide: string
  readonly ms: {
    readonly pop: number
    readonly toast: number
    readonly sheetIn: number
    readonly sheetOut: number
    readonly dialogIn: number
    readonly dialogOut: number
    readonly fade: number
  }
}

export const MOTION: MotionPreset = {
  popIn: 'cubic-bezier(.2,.9,.3,1)',
  springIn: 'cubic-bezier(.2,.9,.25,1.12)',
  exit: 'cubic-bezier(.5,0,.75,0)',
  easeOut: 'cubic-bezier(.2,.8,.2,1)',
  slide: 'cubic-bezier(.2,.9,.25,1)',
  ms: { pop: 170, toast: 400, sheetIn: 440, sheetOut: 260, dialogIn: 200, dialogOut: 180, fade: 250 },
}
