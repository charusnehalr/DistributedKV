import axios from 'axios'
import { toast } from '../components/common/Toast'
import { useSettingsStore } from '../store/useSettingsStore'

// Create an axios instance that always reads the serverURL from settings.
export function createApiClient() {
  const { serverURL, setSessionId } = useSettingsStore.getState()

  const client = axios.create({
    baseURL: serverURL,
    timeout: 10_000,
    headers: { 'Content-Type': 'application/json' },
  })

  // Attach session ID on every request.
  client.interceptors.request.use((config) => {
    const sid = useSettingsStore.getState().sessionId
    if (sid) config.headers['X-Session-ID'] = sid
    return config
  })

  // Capture new session IDs and surface errors as toasts.
  client.interceptors.response.use(
    (response) => {
      const sid = response.headers['x-session-id']
      if (sid) setSessionId(sid)
      return response
    },
    (error) => {
      const msg = error.response?.data?.error ?? error.message ?? 'Request failed'
      toast.error(msg)
      return Promise.reject(error)
    },
  )

  return client
}

// Singleton — recreated when settings change.
let _client = createApiClient()
export const api = {
  get client() {
    return _client
  },
  refresh() {
    _client = createApiClient()
  },
}
