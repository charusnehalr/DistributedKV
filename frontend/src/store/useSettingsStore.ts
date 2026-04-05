import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { AppSettings } from '../types'

interface SettingsStore extends AppSettings {
  sessionId: string
  setServerURL: (url: string) => void
  setDefaultConsistency: (c: AppSettings['defaultConsistency']) => void
  setRefreshInterval: (ms: number) => void
  setSessionId: (id: string) => void
}

export const useSettingsStore = create<SettingsStore>()(
  persist(
    (set) => ({
      serverURL:          'http://localhost:8080',
      defaultConsistency: 'quorum',
      refreshInterval:    5000,
      sessionId:          '',
      setServerURL:          (serverURL)          => set({ serverURL }),
      setDefaultConsistency: (defaultConsistency) => set({ defaultConsistency }),
      setRefreshInterval:    (refreshInterval)    => set({ refreshInterval }),
      setSessionId:          (sessionId)          => set({ sessionId }),
    }),
    { name: 'kvstore-settings' },
  ),
)
