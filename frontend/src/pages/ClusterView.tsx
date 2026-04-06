import { useQuery } from '@tanstack/react-query'
import { Server, Activity, GitMerge, Clock } from 'lucide-react'
import { Card } from '../components/common/Card'
import { Badge } from '../components/common/Badge'
import { Spinner } from '../components/common/Spinner'
import { fetchHealth, fetchMerkleRoot } from '../api/cluster'
import { useSettingsStore } from '../store/useSettingsStore'
import type { ClusterHealth } from '../types'

function NodeCard({ node }: { node: { id: string; address: string; status: string; uptime?: number } }) {
  const statusVariant = node.status === 'healthy' ? 'green' : node.status === 'suspected' ? 'yellow' : 'red'
  return (
    <div className="bg-white border border-gray-200 rounded-xl p-4 flex flex-col gap-3 shadow-sm">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Server className="w-5 h-5 text-blue-600" />
          <span className="font-semibold text-gray-900 text-sm">{node.id}</span>
        </div>
        <Badge variant={statusVariant} dot>{node.status}</Badge>
      </div>
      <div className="text-xs text-gray-500 space-y-1">
        <div className="flex justify-between">
          <span>Address</span>
          <code className="font-mono">{node.address}</code>
        </div>
        {node.uptime !== undefined && (
          <div className="flex justify-between">
            <span>Uptime</span>
            <span>{formatUptime(node.uptime)}</span>
          </div>
        )}
      </div>
    </div>
  )
}

function HashRingViz({ nodes }: { nodes: string[] }) {
  const cx = 120, cy = 120, r = 90
  const count = Math.max(nodes.length, 1)

  return (
    <svg viewBox="0 0 240 240" className="w-full max-w-xs mx-auto">
      {/* Ring */}
      <circle cx={cx} cy={cy} r={r} fill="none" stroke="#e5e7eb" strokeWidth="2" strokeDasharray="6 3" />
      {/* Center label */}
      <text x={cx} y={cy - 6}  textAnchor="middle" className="text-xs fill-gray-400" fontSize="10">Hash Ring</text>
      <text x={cx} y={cy + 10} textAnchor="middle" className="text-xs fill-gray-500 font-bold" fontSize="12">{nodes.length} nodes</text>

      {nodes.map((id, i) => {
        const angle = (2 * Math.PI * i) / count - Math.PI / 2
        const x = cx + r * Math.cos(angle)
        const y = cy + r * Math.sin(angle)
        const colors = ['#3b82f6', '#8b5cf6', '#10b981', '#f59e0b', '#ef4444']
        const color = colors[i % colors.length]
        return (
          <g key={id}>
            <circle cx={x} cy={y} r={14} fill={color} fillOpacity={0.15} stroke={color} strokeWidth={1.5} />
            <text x={x} y={y + 4} textAnchor="middle" fontSize="9" fill={color} fontWeight="600">{id.replace('node-', 'N')}</text>
          </g>
        )
      })}
    </svg>
  )
}

function formatUptime(secs: number) {
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m`
  return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
}

export function ClusterView() {
  const { refreshInterval } = useSettingsStore()

  const healthQuery = useQuery({
    queryKey: ['health'],
    queryFn: fetchHealth,
    refetchInterval: refreshInterval,
    retry: false,
  })

  const merkleQuery = useQuery({
    queryKey: ['merkle'],
    queryFn: fetchMerkleRoot,
    refetchInterval: refreshInterval,
    retry: false,
  })

  const health = healthQuery.data as ClusterHealth | undefined

  // Use the members list returned by the backend (includes all gossip-discovered nodes).
  const nodes = health?.members?.map(m => ({
    id: m.id,
    address: m.address,
    status: m.status as string,
    uptime: m.id === health.node_id ? health.uptime_seconds : undefined,
  })) ?? []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Cluster View</h1>
        <p className="mt-1 text-sm text-gray-500">Live topology, node health, and replication state.</p>
      </div>

      {healthQuery.isLoading && (
        <div className="flex items-center gap-2 text-gray-500"><Spinner /> Loading cluster state…</div>
      )}

      {healthQuery.isError && (
        <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-700">
          Cannot reach server. Make sure the backend is running at the configured URL.
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Ring visualization */}
        <Card title="Hash Ring" className="lg:col-span-1">
          <HashRingViz nodes={nodes.map(n => n.id)} />
        </Card>

        {/* Node cards */}
        <div className="lg:col-span-2 space-y-4">
          <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Nodes</h2>
          {nodes.length === 0 && !healthQuery.isLoading ? (
            <p className="text-sm text-gray-400">No nodes detected.</p>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {nodes.map(n => <NodeCard key={n.id} node={n} />)}
            </div>
          )}
        </div>
      </div>

      {/* Replication info */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card padding="md">
          <div className="flex items-center gap-3">
            <GitMerge className="w-8 h-8 text-blue-500 shrink-0" />
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wide">Replication</p>
              <p className="text-lg font-bold text-gray-900">N=3 R=2 W=2</p>
            </div>
          </div>
        </Card>

        <Card padding="md">
          <div className="flex items-center gap-3">
            <Activity className="w-8 h-8 text-green-500 shrink-0" />
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wide">Cluster Status</p>
              <p className="text-lg font-bold text-gray-900">
                {health ? 'Online' : '—'}
              </p>
            </div>
          </div>
        </Card>

        <Card padding="md">
          <div className="flex items-center gap-3">
            <Clock className="w-8 h-8 text-purple-500 shrink-0" />
            <div>
              <p className="text-xs text-gray-500 uppercase tracking-wide">Merkle Root</p>
              <p className="text-sm font-mono text-gray-700 truncate">
                {merkleQuery.data ? merkleQuery.data.slice(0, 12) + '…' : '—'}
              </p>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
