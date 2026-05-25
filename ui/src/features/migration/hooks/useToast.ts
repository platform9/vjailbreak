import * as React from 'react'

const { useState, useCallback } = React

export function useToast() {
  const [toastOpen, setToastOpen] = useState(false)
  const [toastMessage, setToastMessage] = useState('')
  const [toastSeverity, setToastSeverity] = useState<'success' | 'error' | 'warning' | 'info'>(
    'success'
  )

  const showToast = useCallback(
    (message: string, severity: 'success' | 'error' | 'warning' | 'info' = 'success') => {
      setToastMessage(message)
      setToastSeverity(severity)
      setToastOpen(true)
    },
    []
  )

  const handleCloseToast = useCallback(
    (_event?: React.SyntheticEvent | Event, reason?: string) => {
      if (reason === 'clickaway') return
      setToastOpen(false)
    },
    []
  )

  return { toastOpen, toastMessage, toastSeverity, showToast, handleCloseToast }
}
