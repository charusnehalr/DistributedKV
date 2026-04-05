import { RouterProvider, createBrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Layout } from './components/layout/Layout'
import { Dashboard }   from './pages/Dashboard'
import { KVManager }   from './pages/KVManager'
import { ClusterView } from './pages/ClusterView'
import { MetricsPage } from './pages/MetricsPage'
import { Settings }    from './pages/Settings'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 2000 } },
})

const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true,      element: <Dashboard />   },
      { path: 'kv',       element: <KVManager />   },
      { path: 'cluster',  element: <ClusterView /> },
      { path: 'metrics',  element: <MetricsPage /> },
      { path: 'settings', element: <Settings />    },
    ],
  },
])

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
