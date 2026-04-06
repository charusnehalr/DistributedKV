import { useQuery } from '@tanstack/react-query'
import {
  AreaChart, Area, BarChart, Bar,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import { useRef } from 'react'
import { Card } from '../components/common/Card'
import { Spinner } from '../components/common/Spinner'
import { fetchPrometheusMetrics } from '../api/metrics'
import { useSettingsStore } from '../store/useSettingsStore'

// Keep a rolling window of the last 20 samples.
const MAX_SAMPLES = 20

type MetricSample = { time: string; [key: string]: string | number }

function useRollingMetrics(refreshMs: number) {
  const samplesRef = useRef<MetricSample[]>([])

  const query = useQuery({
    queryKey: ['prometheus'],
    queryFn: async () => {
      const raw = await fetchPrometheusMetrics()
      const sample: MetricSample = { ...raw, time: new Date().toLocaleTimeString() }
      samplesRef.current = [...samplesRef.current.slice(-(MAX_SAMPLES - 1)), sample]
      return [...samplesRef.current]
    },
    refetchInterval: refreshMs,
    retry: false,
  })

  return { samples: query.data ?? samplesRef.current, isLoading: query.isLoading, isError: query.isError }
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="bg-white border border-gray-200 rounded-xl px-5 py-4 shadow-sm">
      <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">{label}</p>
      <p className={`text-2xl font-bold ${color}`}>{value.toLocaleString()}</p>
    </div>
  )
}

export function MetricsPage() {
  const { refreshInterval } = useSettingsStore()
  const { samples, isLoading, isError } = useRollingMetrics(refreshInterval)

  const latest = samples[samples.length - 1] ?? {}

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Metrics</h1>
        <p className="mt-1 text-sm text-gray-500">Live Prometheus metrics from the kvstore node.</p>
      </div>

      {isLoading && <div className="flex items-center gap-2 text-gray-500"><Spinner /> Loading metrics…</div>}
      {isError && (
        <div className="bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm text-red-700">
          Cannot fetch metrics. Ensure the backend is running.
        </div>
      )}

      {/* Counters */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatCard label="Puts"    value={num(latest['kvstore_puts_total'])}    color="text-blue-600"  />
        <StatCard label="Gets"    value={num(latest['kvstore_gets_total'])}    color="text-green-600" />
        <StatCard label="Deletes" value={num(latest['kvstore_deletes_total'])} color="text-yellow-600"/>
        <StatCard label="Errors"  value={num(latest['kvstore_errors_total'])}  color="text-red-600"   />
      </div>

      {/* Storage gauges */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Card title="Memtable Size" padding="md">
          <p className="text-3xl font-bold text-purple-600">
            {formatBytes(num(latest['kvstore_memtable_size_bytes']))}
          </p>
        </Card>
        <Card title="WAL Size" padding="md">
          <p className="text-3xl font-bold text-indigo-600">
            {formatBytes(num(latest['kvstore_wal_size_bytes']))}
          </p>
        </Card>
      </div>

      {/* Operations over time */}
      {samples.length > 1 && (
        <Card title="Operations Over Time" description="Rolling window of the last 20 samples">
          <ResponsiveContainer width="100%" height={260}>
            <AreaChart data={samples} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend />
              <Area type="monotone" dataKey="kvstore_puts_total"    name="Puts"    stroke="#3b82f6" fill="#dbeafe" strokeWidth={2} />
              <Area type="monotone" dataKey="kvstore_gets_total"    name="Gets"    stroke="#10b981" fill="#d1fae5" strokeWidth={2} />
              <Area type="monotone" dataKey="kvstore_deletes_total" name="Deletes" stroke="#f59e0b" fill="#fef3c7" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        </Card>
      )}

      {/* Error breakdown */}
      {samples.length > 1 && (
        <Card title="Error Count Over Time">
          <ResponsiveContainer width="100%" height={160}>
            <BarChart data={samples} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Bar dataKey="kvstore_errors_total" name="Errors" fill="#ef4444" radius={[3, 3, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </Card>
      )}
    </div>
  )
}

function num(v: string | number | undefined): number {
  return typeof v === 'number' ? v : 0
}

function formatBytes(b: number) {
  if (b < 1024)       return `${b} B`
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
  return `${(b / 1024 / 1024).toFixed(2)} MB`
}
