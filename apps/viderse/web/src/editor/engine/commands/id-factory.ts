export interface IdFactory { mint(): string; checkpoint?(): unknown; restore?(checkpoint: unknown): void }
export function createIdFactory(): IdFactory { return { mint: () => crypto.randomUUID() } }
export function createSeededIdFactory(seed = 'command'): IdFactory {
  let counter = 0
  return {
    mint: () => `${seed}-${++counter}`,
    checkpoint: () => counter,
    restore: checkpoint => { if (typeof checkpoint !== 'number') throw new Error('Invalid identifier checkpoint'); counter = checkpoint },
  }
}
let active: IdFactory = createIdFactory()
export function mintId(): string { return active.mint() }
export function withIdFactory<T>(ids: IdFactory, fn: () => T): T {
  const previous = active
  active = ids
  try { return fn() } finally { active = previous }
}
