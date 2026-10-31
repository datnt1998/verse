import { create } from 'zustand'
import { devtools } from 'zustand/middleware'
import { temporal } from 'zundo'
import type { Project } from '@editor/types/project'
import type { ProjectFormData } from '../utils/validation'
import { useSettingsStore } from '@editor/features/projects/deps/settings-contract'
import {
  getAllProjects,
  getProject,
  ensureProject,
  createProject as createProjectDB,
  updateProject as updateProjectDB,
  deleteProject as deleteProjectDB,
  duplicateProject as duplicateProjectDB,
} from '@editor/infrastructure/storage'
import { i18n } from '@editor/i18n'

interface ProjectState {
  // Data
  projects: Project[]
  currentProject: Project | null

  // UI State
  isLoading: boolean
  error: string | null

  // Search and filter state
  searchQuery: string
  sortField: 'name' | 'createdAt' | 'updatedAt' | 'resolution'
  sortDirection: 'asc' | 'desc'
  filterResolution?: string
  filterFps?: number
}

interface ProjectActions {
  // CRUD Operations
  loadProjects: () => Promise<void>
  loadProject: (id: string) => Promise<Project | null>
  createProject: (data: ProjectFormData) => Promise<Project>
  updateProject: (id: string, data: Partial<ProjectFormData>) => Promise<Project>
  /** Permanently delete the project on the server. There is no trash. */
  deleteProject: (id: string) => Promise<void>
  /** Server-side duplicate; sidecars and media links follow the copy. */
  duplicateProject: (id: string) => Promise<Project>

  // State management
  setCurrentProject: (project: Project | null) => void
  setSearchQuery: (query: string) => void
  setSortField: (field: ProjectState['sortField']) => void
  setSortDirection: (direction: ProjectState['sortDirection']) => void
  setFilterResolution: (resolution: string | undefined) => void
  setFilterFps: (fps: number | undefined) => void
  clearFilters: () => void

  // Utility
  clearError: () => void
}

export const useProjectStore = create<ProjectState & ProjectActions>()(
  devtools(
    temporal(
      (set, get) => ({
        // Initial state
        projects: [],
        currentProject: null,
        isLoading: false,
        error: null,
        searchQuery: '',
        sortField: 'updatedAt',
        sortDirection: 'desc',
        filterResolution: undefined,
        filterFps: undefined,

        // Load all projects from workspace storage
        loadProjects: async () => {
          set({ isLoading: true, error: null })

          try {
            const projects = await getAllProjects()
            set({ projects, isLoading: false })
          } catch (error) {
            const errorMessage = error instanceof Error ? error.message : 'Failed to load projects'
            set({ error: errorMessage, isLoading: false })
            throw error
          }
        },

        // Load a single project by ID
        loadProject: async (id: string) => {
          set({ isLoading: true, error: null })

          try {
            const project = await getProject(id)

            if (!project) {
              set({ error: `Project not found: ${id}`, isLoading: false })
              return null
            }

            set({ currentProject: project, isLoading: false })
            return project
          } catch (error) {
            const errorMessage = error instanceof Error ? error.message : 'Failed to load project'
            set({ error: errorMessage, isLoading: false })
            throw error
          }
        },

        // Create on the server, which assigns the project ID
        createProject: async (data: ProjectFormData) => {
          set({ error: null })

          try {
            const created = await createProjectDB({
              name: data.name,
              description: data.description,
              width: data.width,
              height: data.height,
              fps: data.fps,
              backgroundColor: data.backgroundColor,
            })
            set((state) => ({ projects: [...state.projects, created], currentProject: created }))
            return created
          } catch (error) {
            const errorMessage = error instanceof Error ? error.message : 'Failed to create project'
            set({ error: errorMessage })
            throw error
          }
        },

        // Update an existing project with optimistic update
        updateProject: async (id: string, data: Partial<ProjectFormData>) => {
          set({ error: null })

          const previousProjects = get().projects
          const currentProject = get().currentProject
          const projectIndex = previousProjects.findIndex((p) => p.id === id)

          // Find the existing project - either in projects array or currentProject
          let existingProject: Project | null = null
          if (projectIndex !== -1) {
            existingProject = previousProjects[projectIndex] ?? null
          } else if (currentProject?.id === id) {
            existingProject = currentProject
          }

          if (!existingProject) {
            set({ error: `Project not found: ${id}` })
            throw new Error(`Project not found: ${id}`)
          }

          const updatedProject: Project = {
            ...existingProject,
            name: data.name ?? existingProject.name,
            description: data.description ?? existingProject.description,
            metadata: {
              width: data.width ?? existingProject.metadata.width,
              height: data.height ?? existingProject.metadata.height,
              fps: data.fps ?? existingProject.metadata.fps,
              backgroundColor: data.backgroundColor ?? existingProject.metadata.backgroundColor,
            },
            updatedAt: Date.now(),
          }

          // Optimistic update - update projects array if project is there
          if (projectIndex !== -1) {
            const optimisticProjects = [...previousProjects]
            optimisticProjects[projectIndex] = updatedProject
            set({ projects: optimisticProjects })
          }

          // Always update currentProject if it matches
          if (currentProject?.id === id) {
            set({ currentProject: updatedProject })
          }

          try {
            // Edits go through the snapshot retained when the project was loaded, never a refetched base.
            await ensureProject(id)
            const updated = await updateProjectDB(id, {
              name: updatedProject.name,
              description: updatedProject.description,
              metadata: updatedProject.metadata,
            })

            // Update current project with DB result
            if (get().currentProject?.id === id) {
              set({ currentProject: updated })
            }

            return updated
          } catch (error) {
            // Rollback on error
            if (projectIndex !== -1) {
              set({ projects: previousProjects })
            }
            if (currentProject?.id === id) {
              set({ currentProject: currentProject })
            }

            const errorMessage = error instanceof Error ? error.message : 'Failed to update project'
            set({ error: errorMessage })
            throw error
          }
        },

        // Permanent server delete — there is no trash.
        deleteProject: async (id: string) => {
          set({ error: null })

          const previousProjects = get().projects
          const previousCurrentProject = get().currentProject

          // Optimistic update - remove from state immediately
          set({
            projects: previousProjects.filter((p) => p.id !== id),
            currentProject: previousCurrentProject?.id === id ? null : previousCurrentProject,
          })

          try {
            await deleteProjectDB(id)
            set((state) => ({
              projects: state.projects.filter((project) => project.id !== id),
              currentProject: state.currentProject?.id === id ? null : state.currentProject,
            }))
          } catch (error) {
            // Rollback on error
            set({ projects: previousProjects, currentProject: previousCurrentProject })

            const errorMessage = error instanceof Error ? error.message : 'Failed to delete project'
            set({ error: errorMessage })
            throw error
          }
        },

        // Duplicate on the server so sidecars and media links follow the copy.
        duplicateProject: async (id: string) => {
          set({ error: null })

          const originalProject = get().projects.find((p) => p.id === id)

          if (!originalProject) {
            set({ error: `Project not found: ${id}` })
            throw new Error(`Project not found: ${id}`)
          }

          try {
            const copy = await duplicateProjectDB(
              id,
              i18n.t('projects.copySuffix', { name: originalProject.name }),
            )
            set((state) => ({ projects: [...state.projects, copy] }))
            return copy
          } catch (error) {
            const errorMessage =
              error instanceof Error ? error.message : 'Failed to duplicate project'
            set({ error: errorMessage })
            throw error
          }
        },

        // State setters
        setCurrentProject: (project) => set({ currentProject: project }),
        setSearchQuery: (query) => set({ searchQuery: query }),
        setSortField: (field) => set({ sortField: field }),
        setSortDirection: (direction) => set({ sortDirection: direction }),
        setFilterResolution: (resolution) => set({ filterResolution: resolution }),
        setFilterFps: (fps) => set({ filterFps: fps }),
        clearFilters: () =>
          set({
            searchQuery: '',
            filterResolution: undefined,
            filterFps: undefined,
          }),
        clearError: () => set({ error: null }),
      }),
      {
        // Zundo options — no static limit; trimmed dynamically via subscription below
        partialize: (state) => {
          // Only include projects in undo/redo history
          // Exclude UI state like loading, error, filters
          return {
            projects: state.projects,
            currentProject: state.currentProject,
          }
        },
      },
    ),
    {
      // Devtools options
      name: 'ProjectStore',
      enabled: (process.env.NODE_ENV !== 'production'),
    },
  ),
)

// Enforce undo history cap on every save (zundo's static `limit` was removed).
useProjectStore.temporal.getState().setOnSave(() => {
  const max = useSettingsStore.getState().maxUndoHistory
  const { pastStates, futureStates } = useProjectStore.temporal.getState()
  if (pastStates.length > max || futureStates.length > max) {
    useProjectStore.temporal.setState({
      pastStates: pastStates.slice(-max),
      futureStates: futureStates.slice(-max),
    })
  }
})

// When maxUndoHistory changes, immediately trim both stacks.
useSettingsStore.subscribe((state, prevState) => {
  if (state.maxUndoHistory !== prevState.maxUndoHistory) {
    const { pastStates, futureStates } = useProjectStore.temporal.getState()
    if (pastStates.length > state.maxUndoHistory || futureStates.length > state.maxUndoHistory) {
      useProjectStore.temporal.setState({
        pastStates: pastStates.slice(-state.maxUndoHistory),
        futureStates: futureStates.slice(-state.maxUndoHistory),
      })
    }
  }
})
