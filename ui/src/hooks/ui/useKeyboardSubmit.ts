import { useCallback, useEffect } from "react"

/**
 * Custom hook to handle keyboard events for drawer components.
 * Allows Enter to submit the form and Escape to close the drawer.
 *
 * @param open - Whether the drawer is open
 * @param isSubmitDisabled - Whether the submit button should be disabled
 * @param onSubmit - Function to call when Enter is pressed
 * @param onClose - Function to call when Escape is pressed
 */
export function useKeyboardSubmit({
  open,
  isSubmitDisabled,
  onSubmit,
  onClose,
}: {
  open: boolean
  isSubmitDisabled: boolean
  onSubmit: () => void
  onClose: () => void
}) {
  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (event.key === "Enter" && !isSubmitDisabled) {
        event.preventDefault()
        onSubmit()
      } else if (event.key === "Escape") {
        event.preventDefault()
        onClose()
      }
    },
    [isSubmitDisabled, onSubmit, onClose]
  )

  useEffect(() => {
    if (open) {
      document.addEventListener("keydown", handleKeyDown)
    }
    return () => {
      document.removeEventListener("keydown", handleKeyDown)
    }
  }, [open, handleKeyDown])
}
