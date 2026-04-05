import { Outlet } from 'react-router-dom'
import { Header } from './Header'
import { ToastContainer } from '../common/Toast'

export function Layout() {
  return (
    <div className="min-h-screen bg-gray-50">
      <Header />
      <main className="max-w-screen-xl mx-auto px-4 py-6">
        <Outlet />
      </main>
      <ToastContainer />
    </div>
  )
}
