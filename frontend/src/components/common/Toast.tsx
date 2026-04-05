import { create } from 'zustand'
import { CheckCircle2, XCircle, AlertTriangle, Info, X } from 'lucide-react'
import { cn } from '../../utils/cn'

type ToastType = 'success' | 'error' | 'warning' | 'info'

interface Toast {
  id: string
  type: ToastType
  message: string
}

interface ToastStore {
  toasts: Toast[]
  add: (type: ToastType, message: string) => void
  remove: (id: string) => void
}

export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],
  add: (type, message) => {
    const id = Math.random().toString(36).slice(2)
    set((s) => ({ toasts: [...s.toasts, { id, type, message }] }))
    setTimeout(() => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })), 4000)
  },
  remove: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))

export const toast = {
  success: (msg: string) => useToastStore.getState().add('success', msg),
  error:   (msg: string) => useToastStore.getState().add('error',   msg),
  warning: (msg: string) => useToastStore.getState().add('warning', msg),
  info:    (msg: string) => useToastStore.getState().add('info',    msg),
}

const icons = {
  success: <CheckCircle2 className="w-5 h-5 text-green-500" />,
  error:   <XCircle      className="w-5 h-5 text-red-500"   />,
  warning: <AlertTriangle className="w-5 h-5 text-yellow-500" />,
  info:    <Info          className="w-5 h-5 text-blue-500"  />,
}

const borders: Record<ToastType, string> = {
  success: 'border-l-green-500',
  error:   'border-l-red-500',
  warning: 'border-l-yellow-500',
  info:    'border-l-blue-500',
}

export function ToastContainer() {
  const { toasts, remove } = useToastStore()
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 w-80">
      {toasts.map((t) => (
        <div key={t.id} className={cn('flex items-start gap-3 bg-white border border-l-4 rounded-lg shadow-lg px-4 py-3', borders[t.type])}>
          {icons[t.type]}
          <p className="flex-1 text-sm text-gray-800">{t.message}</p>
          <button onClick={() => remove(t.id)} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>
      ))}
    </div>
  )
}
