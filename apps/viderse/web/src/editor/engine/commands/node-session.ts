import { CommandExecutor, bindCommandExecutor } from './executor'
import { captureProjectState, commitProjectState, type ProjectState } from './project-state'
import type { IdFactory } from './id-factory'
import { useCompositionNavigationStore } from '../features/timeline/stores/composition-navigation-store'
import { setActiveCompositionId } from '../features/timeline/stores/composition-navigation-active'
export interface NodeCommandSession { executor: CommandExecutor; getState(): ProjectState; dispose(): void }
export function createNodeSession(initial: ProjectState, ids?: IdFactory): NodeCommandSession {
  const previous = captureProjectState()
  const previousNavigation = useCompositionNavigationStore.getState()
  setActiveCompositionId(null)
  useCompositionNavigationStore.setState({ activeCompositionId: null, breadcrumbs: [{ compositionId: null, label: 'Main Timeline' }], stashStack: [], mainHolder: null })
  commitProjectState(initial)
  const executor = new CommandExecutor(ids)
  const unbind = bindCommandExecutor(executor)
  return {
    executor,
    getState: () => {
      const active = captureProjectState()
      const navigation = useCompositionNavigationStore.getState()
      navigation.resetToRoot()
      const persisted = structuredClone(captureProjectState())
      commitProjectState(active)
      useCompositionNavigationStore.setState(navigation)
      setActiveCompositionId(navigation.activeCompositionId)
      return persisted
    },
    dispose: () => {
      unbind()
      commitProjectState(previous)
      useCompositionNavigationStore.setState(previousNavigation)
      setActiveCompositionId(previousNavigation.activeCompositionId)
    },
  }
}
