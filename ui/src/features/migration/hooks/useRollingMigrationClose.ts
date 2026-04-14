import { useCallback } from 'react'

export function useRollingMigrationClose({
  submitting,
  onClose
}: {
  submitting: boolean
  onClose: () => void
}) {
  const handleClose = useCallback(() => {
    if (!submitting) {
      onClose()
    }
  }, [onClose, submitting])

  return { handleClose }
}
