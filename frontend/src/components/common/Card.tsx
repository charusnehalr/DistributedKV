import React from 'react'
import { cn } from '../../utils/cn'

interface CardProps {
  title?: string
  description?: string
  children: React.ReactNode
  className?: string
  padding?: 'none' | 'sm' | 'md' | 'lg'
}

export function Card({ title, description, children, className, padding = 'md' }: CardProps) {
  const paddings = { none: '', sm: 'p-4', md: 'p-6', lg: 'p-8' }
  return (
    <div className={cn('bg-white rounded-xl border border-gray-200 shadow-sm', className)}>
      {(title || description) && (
        <div className="px-6 pt-5 pb-4 border-b border-gray-100">
          {title && <h3 className="text-base font-semibold text-gray-900">{title}</h3>}
          {description && <p className="mt-0.5 text-sm text-gray-500">{description}</p>}
        </div>
      )}
      <div className={paddings[padding]}>{children}</div>
    </div>
  )
}
