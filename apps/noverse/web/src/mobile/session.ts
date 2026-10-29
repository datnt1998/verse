const listeners = new Set<() => void>()
let pending: { promise: Promise<boolean>; resolve: (value: boolean) => void } | null = null
export const sessionGate = {
  subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener) } },
  isOpen: () => pending !== null,
  request() {
    if (!pending) {
      let finish!: (value: boolean) => void
      const promise = new Promise<boolean>((resolve) => { finish = resolve })
      pending = { promise, resolve: finish }
      listeners.forEach((listener) => listener())
    }
    return pending.promise
  },
  complete() {
    const previous = pending
    pending = null
    previous?.resolve(true)
    listeners.forEach((listener) => listener())
  },
}

let transition: { promise: Promise<boolean>; resolve: (value: boolean) => void } | null = null
/** Suppress only the expected revocation during a deliberate one-cookie account switch. */
export const accountTransition = {
  isActive: () => transition !== null,
  wait: () => transition?.promise ?? Promise.resolve(false),
  begin() {
    if (transition) throw new Error('Đang chuyển tài khoản. Vui lòng đợi.')
    let resolve!: (value: boolean) => void
    const promise = new Promise<boolean>((finish) => { resolve = finish })
    transition = { promise, resolve }
  },
  finish(ok: boolean) { const previous = transition; transition = null; previous?.resolve(ok) },
}
