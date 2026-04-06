// ---- KV Store types ----
export interface KVEntry {
  key: string
  value: string
  timestamp: number
  read_your_write?: boolean
  monotonic?: boolean
}

export interface ScanResult {
  count: number
  entries: Array<{ value: string; timestamp: number }>
}

// ---- Cluster types ----
export type NodeStatus = 'healthy' | 'suspected' | 'failed'

export interface ClusterNode {
  id: string
  address: string
  status: NodeStatus
  keys?: number
  lastSeen?: number
}

export interface ClusterHealth {
  status: string
  node_id: string
  uptime_seconds: number
  members: ClusterNode[]
}

// ---- Metrics types ----
export interface MetricSample {
  time: string
  value: number
}

export interface NodeMetrics {
  puts_total: number
  gets_total: number
  deletes_total: number
  errors_total: number
  memtable_size_bytes: number
  wal_size_bytes: number
}

// ---- API response envelope ----
export interface APIResponse<T> {
  data?: T
  error?: string
}

// ---- Settings ----
export interface AppSettings {
  serverURL: string
  defaultConsistency: 'one' | 'quorum' | 'all'
  refreshInterval: number
}
