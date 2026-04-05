import { api } from './client'

// Fetches raw Prometheus text and parses specific metric values.
export async function fetchPrometheusMetrics(): Promise<Record<string, number>> {
  const res = await api.client.get('/metrics', {
    headers: { Accept: 'text/plain' },
    responseType: 'text',
  })

  const text: string = res.data
  const result: Record<string, number> = {}

  const wanted = [
    'kvstore_puts_total',
    'kvstore_gets_total',
    'kvstore_deletes_total',
    'kvstore_errors_total',
    'kvstore_memtable_size_bytes',
    'kvstore_wal_size_bytes',
  ]

  for (const line of text.split('\n')) {
    if (line.startsWith('#')) continue
    for (const name of wanted) {
      if (line.startsWith(name)) {
        const parts = line.split(' ')
        const val = parseFloat(parts[parts.length - 1])
        if (!isNaN(val)) {
          // Accumulate counter vectors (e.g. errors_total{op="put"})
          result[name] = (result[name] ?? 0) + val
        }
      }
    }
  }
  return result
}
