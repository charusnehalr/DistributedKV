import { useQuery } from '@tanstack/react-query'
import { Database, Activity, Server, GitBranch, ArrowRight } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Card } from '../components/common/Card'
import { Badge } from '../components/common/Badge'
import { Spinner } from '../components/common/Spinner'
import { fetchHealth } from '../api/cluster'
import { fetchPrometheusMetrics } from '../api/metrics'
import { useSettingsStore } from '../store/useSettingsStore'

function StatTile({
  icon,
  label,
  value,
  sub,
  iconColor,
}: {
  icon: React.ReactNode
  label: string
  value: string | number
  sub?: string
  iconColor: string
}) {
  return (
    <div className="bg-white border border-gray-200 rounded-xl p-5 shadow-sm flex items-start gap-4">
      <div className={`p-2.5 rounded-lg ${iconColor}`}>{icon}</div>
      <div>
        <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
        <p className="text-2xl font-bold text-gray-900 mt-0.5">{value}</p>
        {sub && <p className="text-xs text-gray-400 mt-0.5">{sub}</p>}
      </div>
    </div>
  )
}

function QuickAction({ to, label, description }: { to: string; label: string; description: string }) {
  return (
    <Link to={to} className="flex items-center justify-between p-4 bg-white border border-gray-200 rounded-xl hover:border-blue-300 hover:shadow-sm transition-all group">
      <div>
        <p className="font-medium text-gray-900 text-sm">{label}</p>
        <p className="text-xs text-gray-500">{description}</p>
      </div>
      <ArrowRight className="w-4 h-4 text-gray-300 group-hover:text-blue-600 transition-colors" />
    </Link>
  )
}

export function Dashboard() {
  const { refreshInterval, serverURL } = useSettingsStore()

  const healthQuery = useQuery({
    queryKey: ['health'],
    queryFn: fetchHealth,
    refetchInterval: refreshInterval,
    retry: false,
  })

  const metricsQuery = useQuery({
    queryKey: ['prometheus'],
    queryFn: fetchPrometheusMetrics,
    refetchInterval: refreshInterval,
    retry: false,
  })

  const health = healthQuery.data
  const metrics = metricsQuery.data ?? {}
  const isOnline = !!health

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="mt-1 text-sm text-gray-500">Overview of your distributed KV cluster.</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={isOnline ? 'green' : 'red'} dot>
            {isOnline ? 'Online' : 'Offline'}
          </Badge>
          <span className="text-xs text-gray-400">{serverURL}</span>
        </div>
      </div>

      {healthQuery.isLoading && (
        <div className="flex items-center gap-2 text-gray-500 text-sm"><Spinner /> Connecting…</div>
      )}

      {/* Stat tiles */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatTile
          icon={<Server className="w-5 h-5 text-blue-600" />}
          label="Node"
          value={health?.node_id ?? '—'}
          sub={isOnline ? 'Connected' : 'Unreachable'}
          iconColor="bg-blue-50"
        />
        <StatTile
          icon={<Activity className="w-5 h-5 text-green-600" />}
          label="Uptime"
          value={health ? formatUptime(health.uptime_seconds) : '—'}
          iconColor="bg-green-50"
        />
        <StatTile
          icon={<Database className="w-5 h-5 text-purple-600" />}
          label="Total Ops"
          value={((metrics['kvstore_puts_total'] ?? 0) + (metrics['kvstore_gets_total'] ?? 0) + (metrics['kvstore_deletes_total'] ?? 0)).toLocaleString()}
          sub="puts + gets + deletes"
          iconColor="bg-purple-50"
        />
        <StatTile
          icon={<GitBranch className="w-5 h-5 text-orange-600" />}
          label="Errors"
          value={(metrics['kvstore_errors_total'] ?? 0).toLocaleString()}
          iconColor="bg-orange-50"
        />
      </div>

      {/* Quick actions */}
      <Card title="Quick Actions" padding="md">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <QuickAction to="/kv"      label="Open KV Explorer"   description="PUT, GET, DELETE, SCAN keys" />
          <QuickAction to="/cluster" label="View Cluster"       description="Hash ring & node health"     />
          <QuickAction to="/metrics" label="Live Metrics"       description="Prometheus counters & charts" />
          <QuickAction to="/settings"label="Settings"           description="Server URL & consistency"    />
        </div>
      </Card>

      {/* System info */}
      {health && (
        <Card title="System Info" padding="md">
          <dl className="grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
            {[
              { label: 'Node ID',     value: health.node_id },
              { label: 'Status',      value: health.status },
              { label: 'Uptime',      value: formatUptime(health.uptime_seconds) },
              { label: 'Consistency', value: 'Quorum (N=3 R=2 W=2)' },
            ].map(({ label, value }) => (
              <div key={label}>
                <dt className="text-xs text-gray-500 uppercase tracking-wide">{label}</dt>
                <dd className="mt-1 font-medium text-gray-900">{value}</dd>
              </div>
            ))}
          </dl>
        </Card>
      )}
    </div>
  )
}

function formatUptime(secs: number) {
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`
  return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
}
