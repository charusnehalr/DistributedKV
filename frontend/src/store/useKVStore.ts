import { create } from 'zustand'
import type { KVEntry } from '../types'

interface KVStore {
  history: KVEntry[]        // recent GET results
  addHistory: (entry: KVEntry) => void
  clearHistory: () => void
}

export const useKVStore = create<KVStore>((set) => ({
  history: [],
  addHistory: (entry) =>
    set((s) => ({ history: [entry, ...s.history].slice(0, 50) })),
  clearHistory: () => set({ history: [] }),
}))
