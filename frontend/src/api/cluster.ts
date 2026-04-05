import { api } from './client'
import type { ClusterHealth } from '../types'

export async function fetchHealth(): Promise<ClusterHealth> {
  const res = await api.client.get('/api/v1/health')
  return res.data.data as ClusterHealth
}

export async function fetchMerkleRoot(): Promise<string> {
  const res = await api.client.get('/api/v1/admin/merkle')
  return res.data.data?.root_hash ?? ''
}
