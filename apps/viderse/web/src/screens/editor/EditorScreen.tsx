import type { Route } from '../../router'
import { Editor } from '../../editor/v13/Editor'

export function EditorScreen({ route }: { route: Extract<Route, { name: 'editor' }> }) {
  return <Editor key={route.projectId} projectId={route.projectId} />
}
