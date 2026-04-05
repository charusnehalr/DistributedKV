import { Database } from 'lucide-react'
import { NavLink } from 'react-router-dom'
import { cn } from '../../utils/cn'

const navLinks = [
  { to: '/',        label: 'Dashboard' },
  { to: '/kv',      label: 'Key-Value' },
  { to: '/cluster', label: 'Cluster'   },
  { to: '/metrics', label: 'Metrics'   },
  { to: '/settings',label: 'Settings'  },
]

export function Header() {
  return (
    <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
      <div className="max-w-screen-xl mx-auto px-4 h-14 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Database className="w-6 h-6 text-blue-600" />
          <span className="font-bold text-lg text-gray-900">DistributedKV</span>
        </div>
        <nav className="hidden md:flex items-center gap-1">
          {navLinks.map(({ to, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                cn('px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-blue-50 text-blue-700'
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                )
              }
            >
              {label}
            </NavLink>
          ))}
        </nav>
      </div>
    </header>
  )
}
