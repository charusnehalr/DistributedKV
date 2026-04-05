import { cn } from '../../utils/cn'

type BadgeVariant = 'green' | 'yellow' | 'red' | 'blue' | 'gray' | 'purple'

interface BadgeProps {
  variant?: BadgeVariant
  children: React.ReactNode
  className?: string
  dot?: boolean
}

const styles: Record<BadgeVariant, string> = {
  green:  'bg-green-100  text-green-800',
  yellow: 'bg-yellow-100 text-yellow-800',
  red:    'bg-red-100    text-red-800',
  blue:   'bg-blue-100   text-blue-800',
  gray:   'bg-gray-100   text-gray-700',
  purple: 'bg-purple-100 text-purple-800',
}

const dotStyles: Record<BadgeVariant, string> = {
  green:  'bg-green-500',
  yellow: 'bg-yellow-500',
  red:    'bg-red-500',
  blue:   'bg-blue-500',
  gray:   'bg-gray-400',
  purple: 'bg-purple-500',
}

export function Badge({ variant = 'gray', children, className, dot }: BadgeProps) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium', styles[variant], className)}>
      {dot && <span className={cn('w-1.5 h-1.5 rounded-full', dotStyles[variant])} />}
      {children}
    </span>
  )
}
