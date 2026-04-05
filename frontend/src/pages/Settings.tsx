import { useState } from 'react'
import { Card } from '../components/common/Card'
import { Input } from '../components/common/Input'
import { Button } from '../components/common/Button'
import { toast } from '../components/common/Toast'
import { useSettingsStore } from '../store/useSettingsStore'
import { api } from '../api/client'

export function Settings() {
  const {
    serverURL, defaultConsistency, refreshInterval, sessionId,
    setServerURL, setDefaultConsistency, setRefreshInterval,
  } = useSettingsStore()

  const [url, setUrl] = useState(serverURL)
  const [interval, setInterval_] = useState(String(refreshInterval / 1000))

  function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setServerURL(url)
    setRefreshInterval(Number(interval) * 1000)
    api.refresh()
    toast.success('Settings saved')
  }

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="mt-1 text-sm text-gray-500">Configure the dashboard connection and behaviour.</p>
      </div>

      <form onSubmit={handleSave} className="space-y-4">
        <Card title="Connection" padding="md">
          <div className="space-y-4">
            <Input
              label="Server URL"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="http://localhost:8080"
              hint="Base URL of the kvstore HTTP API"
            />
            <Input
              label="Refresh Interval (seconds)"
              type="number"
              min={1}
              max={60}
              value={interval}
              onChange={(e) => setInterval_(e.target.value)}
              hint="How often live data is polled (1–60 s)"
            />
          </div>
        </Card>

        <Card title="Default Consistency Level" padding="md">
          <div className="space-y-2">
            {(['one', 'quorum', 'all'] as const).map((level) => (
              <label key={level} className="flex items-start gap-3 cursor-pointer">
                <input
                  type="radio"
                  name="consistency"
                  value={level}
                  checked={defaultConsistency === level}
                  onChange={() => setDefaultConsistency(level)}
                  className="mt-0.5"
                />
                <div>
                  <span className="font-medium text-sm text-gray-900 capitalize">{level}</span>
                  <p className="text-xs text-gray-500">
                    {level === 'one'    && 'Read/write from 1 replica — fastest, lowest durability'}
                    {level === 'quorum' && 'Read/write from majority (N/2+1) — balanced (default)'}
                    {level === 'all'    && 'Read/write from all N replicas — strongest, slowest'}
                  </p>
                </div>
              </label>
            ))}
          </div>
        </Card>

        <Card title="Session" padding="md">
          <div className="text-sm text-gray-600 space-y-2">
            <p>Current session ID (read-your-writes tracking):</p>
            <code className="block bg-gray-100 rounded px-3 py-2 text-xs font-mono break-all">
              {sessionId || '(none — will be assigned on first request)'}
            </code>
          </div>
        </Card>

        <Button type="submit" className="w-full">Save Settings</Button>
      </form>
    </div>
  )
}
