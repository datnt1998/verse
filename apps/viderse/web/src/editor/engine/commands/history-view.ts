import { create } from 'zustand'
export interface CommandHistoryView { canUndo: boolean; canRedo: boolean; revision: number }
export const useCommandHistory = create<CommandHistoryView>(() => ({ canUndo: false, canRedo: false, revision: 0 }))
