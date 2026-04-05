import { api } from './client'
import type { KVEntry, ScanResult } from '../types'

export type Consistency = 'one' | 'quorum' | 'all'

export async function kvPut(key: string, value: string, consistency: Consistency = 'quorum'): Promise<void> {
  await api.client.post(`/api/v1/kv?consistency=${consistency}`, { key, value })
}

export async function kvGet(key: string, consistency: Consistency = 'quorum'): Promise<KVEntry> {
  const res = await api.client.get(`/api/v1/kv/${encodeURIComponent(key)}?consistency=${consistency}`)
  return res.data.data as KVEntry
}

export async function kvDelete(key: string, consistency: Consistency = 'quorum'): Promise<void> {
  await api.client.delete(`/api/v1/kv/${encodeURIComponent(key)}?consistency=${consistency}`)
}

export async function kvScan(prefix: string): Promise<ScanResult> {
  const res = await api.client.get(`/api/v1/kv?prefix=${encodeURIComponent(prefix)}`)
  return res.data.data as ScanResult
}
