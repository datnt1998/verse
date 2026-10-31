import { getMediaType, getMimeType, validateMediaFileContent } from './validation'

export interface ExtractedMediaFileEntry {
  file: File
  mediaType: 'video' | 'audio' | 'image' | 'lottie' | 'unknown'
}

export interface ExtractedMediaFileDropResult {
  entries: ExtractedMediaFileEntry[]
  errors: string[]
}

export function formatMediaDropRejectionMessage(errors: string[]): string {
  const count = errors.length
  if (count === 0) return ''

  const examples = errors.slice(0, 3).join('; ')
  const overflow = count > 3 ? `; and ${count - 3} more` : ''
  const subject =
    count === 1 ? '1 dropped item was rejected' : `${count} dropped items were rejected`
  return `${subject}: ${examples}${overflow}.`
}

/**
 * Collect the media files of a drop. Entries come from `DataTransferItem.webkitGetAsEntry()`
 * (to tell folders from files) with `getAsFile()` supplying the disk-backed `File` that is
 * later uploaded; nothing about the drop is persisted.
 */
export async function extractValidMediaFileEntriesFromDataTransfer(
  dataTransfer: DataTransfer,
): Promise<ExtractedMediaFileDropResult> {
  const entries: ExtractedMediaFileEntry[] = []
  const errors: string[] = []

  // Items must be read synchronously; DataTransfer is cleared once the event handler yields.
  const dropped: Array<{ file: File | null; isDirectory: boolean }> = []
  for (const item of Array.from(dataTransfer.items)) {
    if (item.kind !== 'file') continue
    dropped.push({
      file: item.getAsFile(),
      isDirectory: item.webkitGetAsEntry?.()?.isDirectory === true,
    })
  }

  for (const { file, isDirectory } of dropped) {
    if (!file) continue

    if (isDirectory) {
      errors.push(`${file.name}: folders are not supported yet. Drop media files directly.`)
      continue
    }

    try {
      const validation = await validateMediaFileContent(file)
      if (!validation.valid) {
        errors.push(`${file.name}: ${validation.error}`)
        continue
      }

      entries.push({ file, mediaType: getMediaType(getMimeType(file)) })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to read file'
      errors.push(`${file.name}: ${message}`)
    }
  }

  return { entries, errors }
}
