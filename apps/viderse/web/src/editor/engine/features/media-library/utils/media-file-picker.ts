export const MEDIA_FILE_PICKER_TYPES = [
  {
    description: 'Media files',
    accept: {
      'video/*': ['.mp4', '.webm', '.mov', '.avi', '.mkv'],
      'audio/*': ['.mp3', '.wav', '.ogg', '.m4a', '.aac'],
      'image/*': ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.svg'],
      'application/lottie+json': ['.json', '.lottie'],
    },
  },
] satisfies FilePickerAcceptType[]

const FORMAT_LABEL_OVERRIDES: Record<string, string> = {
  webm: 'WebM',
  webp: 'WebP',
}

export function getSupportedMediaFormatLabels(): string[] {
  const extensions = Object.values(MEDIA_FILE_PICKER_TYPES[0]?.accept ?? {}).flat()
  return extensions.map((extension) => {
    const normalized = extension.replace(/^\./, '')
    return FORMAT_LABEL_OVERRIDES[normalized] ?? normalized.toUpperCase()
  })
}

/** Imported files are uploaded; the editor never retains permission handles. */
export function pickMediaFiles(options: { multiple?: boolean } = {}): Promise<File[]> {
  const { promise, resolve, reject } = Promise.withResolvers<File[]>()
  const input = document.createElement('input')
  input.type = 'file'
  input.multiple = options.multiple ?? true
  input.accept = Object.values(MEDIA_FILE_PICKER_TYPES[0].accept).flat().join(',')
  input.hidden = true
  const cleanup = () => input.remove()
  input.addEventListener('change', () => {
    const files = Array.from(input.files ?? [])
    cleanup()
    if (files.length) resolve(files)
    else reject(new DOMException('Đã hủy chọn media', 'AbortError'))
  }, { once: true })
  input.addEventListener('cancel', () => {
    cleanup()
    reject(new DOMException('Đã hủy chọn media', 'AbortError'))
  }, { once: true })
  document.body.append(input)
  input.click()
  return promise
}
