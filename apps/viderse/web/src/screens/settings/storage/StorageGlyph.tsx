export function StorageGlyph({ kind, size = 20 }: { kind: 'drive' | 'cloud' | 'lock'; size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === 'drive' ? <><rect x="3" y="3" width="18" height="7" rx="2.5" /><rect x="3" y="14" width="18" height="7" rx="2.5" /><path d="M7 6.5h.01M7 17.5h.01M12 6.5h5M12 17.5h5" /></> : kind === 'cloud' ? <path d="M6.5 19a4.5 4.5 0 0 1-.9-8.9 6.5 6.5 0 0 1 12.5-1.3A5.1 5.1 0 0 1 18 19Z" /> : <><rect x="5" y="10" width="14" height="11" rx="2.5" /><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v3" /></>}
  </svg>
}
