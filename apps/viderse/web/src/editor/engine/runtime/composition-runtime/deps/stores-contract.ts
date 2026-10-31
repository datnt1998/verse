/**
 * Adapter exports for shared store dependencies.
 * Composition runtime modules should import store hooks/types from here.
 */

export { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
export { useGizmoStore } from '@editor/features/preview/stores/gizmo-store'
export { useItemGizmoPreview } from '@editor/features/preview/stores/use-item-gizmo-preview'
export type { ItemPropertiesPreview } from '@editor/features/preview/stores/gizmo-store'
export { useCornerPinStore } from '@editor/features/preview/stores/corner-pin-store'
export { useMaskEditorStore } from '@editor/features/preview/stores/mask-editor-store'
export { usePlaybackStore } from '@editor/shared/state/playback'
export { useTimelineStore } from '@editor/features/timeline/stores/timeline-store'
export { useCompositionsStore } from '@editor/features/timeline/stores/compositions-store'
export { useDebugStore } from '@editor/features/editor/stores/debug-store'
