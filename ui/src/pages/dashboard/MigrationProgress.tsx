import React, { useMemo, useState, useEffect } from "react"
import { CircularProgress, styled, Typography, Box } from "@mui/material"
import CheckCircleOutlineIcon from "@mui/icons-material/CheckCircleOutline"
import ErrorOutlineIcon from "@mui/icons-material/ErrorOutline"
import HourglassBottomIcon from "@mui/icons-material/HourglassBottom"
import Snackbar from "@mui/material/Snackbar"
import MuiAlert from "@mui/material/Alert"
import { Phase } from "src/api/migrations/model"

const ProgressContainer = styled(Box)({
  display: "flex",
  alignItems: "center",
  height: "100%",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
  maxWidth: "100%",
  overflow: "hidden",
})

interface MigrationProgressProps {
  phase: Phase
  progressText: string
}

export default function MigrationProgress({
  progressText,
  phase
}: MigrationProgressProps) {
  // Get phase from the most recent condition

  // Update the statusIcon logic to use the Phase enum
  const statusIcon = useMemo(() => {
    if (phase === Phase.Succeeded) {
      return <CheckCircleOutlineIcon style={{ color: "green" }} />
    } else if ([
      Phase.Validating,
      Phase.AwaitingDataCopyStart,
      Phase.CopyingBlocks,
      Phase.CopyingChangedBlocks,
      Phase.ConvertingDisk,
      Phase.AwaitingCutOverStartTime,
      Phase.AwaitingAdminCutOver
    ].includes(phase as Phase)) {
      return <CircularProgress size={20} style={{ marginRight: 3 }} />
    } else if (phase === Phase.Failed) {
      return <ErrorOutlineIcon style={{ color: "red" }} />
    } else {
      return <HourglassBottomIcon style={{ color: "grey" }} />
    }
  }, [phase])

  // Show error popup if migration is blocked
  const [open, setOpen] = useState(false)

  useEffect(() => {
    // Show the alert if the progress text starts with "Blocked"
    if (progressText && progressText.toLowerCase().startsWith("blocked:")) {
      setOpen(true);
    } else {
      setOpen(false);
    }
  }, [progressText]);

  const handleClose = (_?: React.SyntheticEvent | Event, reason?: string) => {
    if (reason === 'clickaway') {
      return
    }
    setOpen(false)
  }

  return (
    <>
      <ProgressContainer>
        {statusIcon}
        <Typography variant="body2" sx={{ ml: 2 }}>
          {progressText}
        </Typography>
      </ProgressContainer>
      <Snackbar open={open} autoHideDuration={7000} onClose={handleClose} anchorOrigin={{ vertical: 'top', horizontal: 'center' }}>
        <MuiAlert onClose={handleClose} severity="error" elevation={6} variant="filled">
          {progressText}
        </MuiAlert>
      </Snackbar>
    </>
  )
}
