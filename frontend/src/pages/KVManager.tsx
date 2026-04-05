import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Search, Plus, Trash2, List, Clock } from 'lucide-react'
import { Card } from '../components/common/Card'
import { Button } from '../components/common/Button'
import { Input } from '../components/common/Input'
import { Badge } from '../components/common/Badge'

import { toast } from '../components/common/Toast'
import { useKVStore } from '../store/useKVStore'
import { useSettingsStore } from '../store/useSettingsStore'
import { kvPut, kvGet, kvDelete, kvScan } from '../api/kv'
import type { Consistency } from '../api/kv'
import type { KVEntry } from '../types'

type Op = 'put' | 'get' | 'delete' | 'scan'

export function KVManager() {
  const [op, setOp] = useState<Op>('get')
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const [prefix, setPrefix] = useState('')
  const [result, setResult] = useState<KVEntry | null>(null)
  const [scanResults, setScanResults] = useState<{ value: string; timestamp: number }[]>([])

  const { defaultConsistency } = useSettingsStore()
  const { history, addHistory, clearHistory } = useKVStore()

  const consistency = defaultConsistency as Consistency

  const putMutation = useMutation({
    mutationFn: () => kvPut(key, value, consistency),
    onSuccess: () => {
      toast.success(`Key "${key}" stored`)
      setKey(''); setValue('')
    },
  })

  const getMutation = useMutation({
    mutationFn: () => kvGet(key, consistency),
    onSuccess: (data) => {
      setResult(data)
      addHistory(data)
    },
    onError: () => setResult(null),
  })

  const deleteMutation = useMutation({
    mutationFn: () => kvDelete(key, consistency),
    onSuccess: () => {
      toast.success(`Key "${key}" deleted`)
      setKey('')
    },
  })

  const scanMutation = useMutation({
    mutationFn: () => kvScan(prefix),
    onSuccess: (data) => setScanResults(data.entries),
  })

  const ops: { id: Op; label: string; color: string }[] = [
    { id: 'get',    label: 'GET',    color: 'blue'  },
    { id: 'put',    label: 'PUT',    color: 'green' },
    { id: 'delete', label: 'DELETE', color: 'red'   },
    { id: 'scan',   label: 'SCAN',   color: 'purple'},
  ]

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (op === 'put')    putMutation.mutate()
    if (op === 'get')    getMutation.mutate()
    if (op === 'delete') deleteMutation.mutate()
    if (op === 'scan')   scanMutation.mutate()
  }

  const isLoading = putMutation.isPending || getMutation.isPending || deleteMutation.isPending || scanMutation.isPending

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Key-Value Store</h1>
        <p className="mt-1 text-sm text-gray-500">Read and write data with configurable consistency levels.</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Operation panel */}
        <div className="lg:col-span-2 space-y-4">
          <Card padding="md">
            {/* Op tabs */}
            <div className="flex gap-2 mb-5">
              {ops.map(({ id, label, color }) => (
                <button
                  key={id}
                  onClick={() => { setOp(id); setResult(null); setScanResults([]) }}
                  className={`px-3 py-1.5 rounded-md text-sm font-mono font-semibold transition-colors ${
                    op === id
                      ? color === 'blue'   ? 'bg-blue-600 text-white'
                      : color === 'green'  ? 'bg-green-600 text-white'
                      : color === 'red'    ? 'bg-red-600 text-white'
                      : 'bg-purple-600 text-white'
                      : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              {op !== 'scan' && (
                <Input
                  label="Key"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  placeholder="e.g. user:123"
                  required
                />
              )}
              {op === 'put' && (
                <Input
                  label="Value"
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  placeholder="e.g. Alice"
                  required
                />
              )}
              {op === 'scan' && (
                <Input
                  label="Prefix"
                  value={prefix}
                  onChange={(e) => setPrefix(e.target.value)}
                  placeholder="e.g. user:"
                />
              )}

              <Button type="submit" loading={isLoading} className="w-full">
                {op === 'get'    && <><Search  className="w-4 h-4" /> Fetch</>}
                {op === 'put'    && <><Plus    className="w-4 h-4" /> Store</>}
                {op === 'delete' && <><Trash2  className="w-4 h-4" /> Delete</>}
                {op === 'scan'   && <><List    className="w-4 h-4" /> Scan</>}
              </Button>
            </form>
          </Card>

          {/* GET result */}
          {result && op === 'get' && (
            <Card title="Result" padding="md">
              <div className="space-y-3">
                <div className="flex justify-between items-start">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Key</span>
                  <code className="text-sm font-mono bg-gray-100 px-2 py-0.5 rounded">{result.key}</code>
                </div>
                <div className="flex justify-between items-start">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Value</span>
                  <code className="text-sm font-mono bg-blue-50 text-blue-900 px-2 py-0.5 rounded">{result.value}</code>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Timestamp</span>
                  <span className="text-xs text-gray-600">{new Date(result.timestamp / 1e6).toLocaleString()}</span>
                </div>
                <div className="flex gap-2 pt-1">
                  <Badge variant={result.read_your_write ? 'green' : 'yellow'} dot>
                    {result.read_your_write ? 'RYW ✓' : 'RYW ?'}
                  </Badge>
                  <Badge variant={result.monotonic ? 'green' : 'yellow'} dot>
                    {result.monotonic ? 'Monotonic ✓' : 'Monotonic ?'}
                  </Badge>
                </div>
              </div>
            </Card>
          )}

          {/* SCAN results */}
          {scanResults.length > 0 && op === 'scan' && (
            <Card title={`Scan Results (${scanResults.length})`} padding="none">
              <table className="w-full text-sm">
                <thead className="bg-gray-50 border-b border-gray-200">
                  <tr>
                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">#</th>
                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Value</th>
                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Timestamp</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {scanResults.map((r, i) => (
                    <tr key={i} className="hover:bg-gray-50">
                      <td className="px-4 py-2 text-gray-400">{i + 1}</td>
                      <td className="px-4 py-2 font-mono">{r.value}</td>
                      <td className="px-4 py-2 text-gray-500 text-xs">{new Date(r.timestamp / 1e6).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          )}
        </div>

        {/* History panel */}
        <Card title="Recent Reads" padding="none" className="self-start">
          <div className="flex justify-end px-4 py-2 border-b border-gray-100">
            <button onClick={clearHistory} className="text-xs text-gray-400 hover:text-gray-600">Clear</button>
          </div>
          {history.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-10 text-gray-400">
              <Clock className="w-8 h-8 mb-2" />
              <p className="text-sm">No reads yet</p>
            </div>
          ) : (
            <ul className="divide-y divide-gray-100 max-h-96 overflow-y-auto">
              {history.map((h, i) => (
                <li key={i} className="px-4 py-3 hover:bg-gray-50 cursor-pointer" onClick={() => { setKey(h.key); setOp('get') }}>
                  <div className="flex justify-between items-center">
                    <code className="text-xs font-mono text-gray-800 truncate">{h.key}</code>
                    <Badge variant="gray" className="ml-2 shrink-0 text-xs">{h.value.slice(0, 12)}</Badge>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  )
}
