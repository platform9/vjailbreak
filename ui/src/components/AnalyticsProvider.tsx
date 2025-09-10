import React from 'react'
import { useAnalytics } from '../hooks/useAnalytics'

interface AnalyticsProviderProps {
  children: React.ReactNode
}

export function AnalyticsProvider({ children }: AnalyticsProviderProps) {
  useAnalytics()
  
  return <>{children}</>
}