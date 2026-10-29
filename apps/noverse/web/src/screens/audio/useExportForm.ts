import { useEffect, useState } from 'react'
import { getAudioExportPrefs, saveAudioExportPrefs } from '../../api/client'
import { audioExportOptions, defaultExportPrefs, exportPrefsOf, exportVisibility, type AudioExportForm } from './export'

export function useExportForm(project: string, story: string, open: boolean, initial: Partial<AudioExportForm> = {}) {
  const [form, setForm] = useState<AudioExportForm>(() => ({ ...defaultExportPrefs, gap: true, whenReady: false, ...initial }))
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  useEffect(() => {
    if (!open) return
    const controller = new AbortController()
    setLoading(true)
    void getAudioExportPrefs(project, story, controller.signal).then((prefs) => {
      if (!controller.signal.aborted) { setForm((previous) => ({ ...previous, ...prefs, gap: prefs.gapSec > 0 })); setError(null) }
    }).catch((cause: unknown) => { if (!controller.signal.aborted) setError(cause) })
      .finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [project, story, open])
  const update = <K extends keyof AudioExportForm>(key: K, value: AudioExportForm[K]) => setForm((previous) => ({ ...previous, [key]: value }))
  return { form, setForm, update, loading, error, visible: exportVisibility(form.split === true, form.format),
    save: () => saveAudioExportPrefs(project, story, exportPrefsOf(form)), options: audioExportOptions({ ...form, chapters: form.split ? form.chapters : undefined }) }
}
