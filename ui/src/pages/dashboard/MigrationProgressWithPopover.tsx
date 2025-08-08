import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Stepper,
  Step,
  StepLabel,
  CircularProgress,
  styled,
  Popover,
  Typography,
  Box,
} from "@mui/material"
//Icons
import CheckCircleOutlineIcon from "@mui/icons-material/CheckCircleOutline"
import ErrorOutlineIcon from "@mui/icons-material/ErrorOutline"
import HourglassBottomIcon from "@mui/icons-material/HourglassBottom"
import PauseCircleOutlineIcon from "@mui/icons-material/PauseCircleOutline"
import { Phase, Condition } from "src/api/migrations/model"

// Interfaces
enum Status {
  Pending = "Pending",
  Completed = "Completed",
  InProgress = "InProgress",
  Failed = "Failed",
}

enum StatusType {
  Validated = "Validated",
  DataCopy = "DataCopy",
  Migrated = "Migrated",
}

interface StatusStep {
  type: StatusType
  status: Status
  message: string
  reason?: string
  lastTransitionTime?: string
}

const defaultSteps = {
  [StatusType.Validated]: {
    type: StatusType.Validated,
    status: Status.Pending,
    message: "Validating",
  },
  [StatusType.DataCopy]: {
    type: StatusType.DataCopy,
    status: Status.Pending,
    message: "Copying Data",
  },
  [StatusType.Migrated]: {
    type: StatusType.Migrated,
    status: Status.Pending,
    message: "Migrating",
  },
}

const StepperContainer = styled("div")({
  width: "100%",
  padding: "4px",
  margin: "12px",
})

const ProgressContainer = styled(Box)({
  display: "flex",
  alignItems: "center",
  height: "100%",
  cursor: "pointer"
})

interface MigrationProgressWithPopoverProps {
  phase: Phase | undefined
  conditions: Condition[]
  progressText: string
}

export default function MigrationProgressWithPopover({
  phase,
  conditions,
  progressText
}: MigrationProgressWithPopoverProps) {
  const [steps, setSteps] = useState<StatusStep[]>(Object.values(defaultSteps))
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null)

  // Function to map status to the appropriate label
  const mapStatus = (status) => {
    if (status === "True") return Status.Completed
    if (status === "Unknown") return Status.InProgress
    if (status === "False") return Status.Failed
    return Status.Pending
  }

  // Format steps based on updated conditions
  useEffect(() => {
    const updatedSteps = conditions.reduce((acc, currentStep) => {
      const { type, status } = currentStep

      // Update the step if there's a more recent message or status
      acc[type] = {
        ...currentStep,
        status: mapStatus(status),
      }

      return acc
    }, JSON.parse(JSON.stringify(defaultSteps)))

    // Convert object back to an array of steps
    setSteps(Object.values(updatedSteps))
  }, [defaultSteps, conditions])

  // Update the statusIcon logic to use the new Phase enum
  const statusIcon = useMemo(() => {
    if (phase === Phase.Succeeded) {
      return <CheckCircleOutlineIcon style={{ color: "green" }} />
    } else if (phase === Phase.AwaitingAdminCutOver) {
      return <PauseCircleOutlineIcon style={{ color: "#1976d2" }} />
    } else if ([
      Phase.Validating,
      Phase.AwaitingDataCopyStart,
      Phase.CopyingBlocks,
      Phase.CopyingChangedBlocks,
      Phase.ConvertingDisk,
      Phase.AwaitingCutOverStartTime
    ].includes(phase as Phase)) {
      return <CircularProgress size={20} style={{ marginRight: 3 }} />
    } else if (phase === Phase.Failed) {
      return <ErrorOutlineIcon style={{ color: "red" }} />
    } else {
      return <HourglassBottomIcon style={{ color: "grey" }} />
    }
  }, [phase])

  // Get active step index and active step
  const activeStepIndex: number = useMemo(() => {
    return steps.findIndex(
      (step) =>
        step.status === Status.InProgress || step.status === Status.Failed
    )
  }, [steps, conditions])

  const activeStep: StatusStep = useMemo(
    () => steps[activeStepIndex],
    [steps, activeStepIndex]
  )

  //Popover handlers
  const handlePopoverOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget)
  }

  const handlePopoverClose = () => {
    setAnchorEl(null)
  }

  return (
    <ProgressContainer
      onMouseEnter={handlePopoverOpen}
      onMouseLeave={handlePopoverClose}
    >
      {/* Status icon and phase */}
      {statusIcon}
      <Typography variant="body2" sx={{ ml: 2 }}>
        {progressText}
      </Typography>




      <Popover
        id="mouse-over-popover"
        sx={{ pointerEvents: "none" }}
        open={Boolean(anchorEl)}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: "top",
          horizontal: "left",
        }}
        transformOrigin={{
          vertical: "top",
          horizontal: "left",
        }}
        onClose={handlePopoverClose}
        disableRestoreFocus
      >
        <StepperComponent
          activeStepIndex={activeStepIndex}
          activeStep={activeStep}
          steps={steps}
        />
      </Popover>
    </ProgressContainer>
  )
}

const StepperComponent = ({
  activeStepIndex,
  activeStep,
  steps,
}: {
  steps: StatusStep[]
  activeStepIndex: number
  activeStep: StatusStep
}) => {
  // Return step icon based on the status
  const getStepIcon = useCallback((status: Status) => {
    let stepIcon
    switch (status) {
      case Status.Completed:
        stepIcon = () => <CheckCircleOutlineIcon style={{ color: "green" }} />
        break
      case Status.Pending:
        stepIcon = () => <HourglassBottomIcon style={{ color: "grey" }} />
        break
      case Status.InProgress:
        stepIcon = () => <CircularProgress size={20} sx={{ color: "green" }} />
        break
      case Status.Failed:
        stepIcon = () => <ErrorOutlineIcon style={{ color: "red" }} />
        break
      default:
        break
    }
    return stepIcon
  }, [])

  const lastUpdated = activeStep?.lastTransitionTime
    ? new Date(String(activeStep?.lastTransitionTime)).toLocaleTimeString(
      "en-US"
    )
    : null

  const diffInMinutes = useMemo(() => {
    const localDate: Date = new Date()
    const utcTime: Date = new Date(String(activeStep?.lastTransitionTime))
    const timeDifferenceInMs: number = localDate.getTime() - utcTime.getTime()
    const diffInMinutes: number = Math.floor(timeDifferenceInMs / 1000 / 60)

    return diffInMinutes
  }, [])

  return (
    <StepperContainer>
      {lastUpdated && (
        <Typography>
          {`Last Updated: ${lastUpdated} (${diffInMinutes} mintues ago)`}
        </Typography>
      )}
      <Stepper activeStep={activeStepIndex} orientation="vertical">
        {steps.map((step, index) => (
          <Step
            key={`${index}+${String(step.type)}`}
            completed={[Status.Completed, Status.Failed].includes(step.status)}
          >
            <StepLabel StepIconComponent={getStepIcon(step.status)}>
              <Typography
                sx={{
                  color: step.status === Status.Failed ? "red" : "default",
                }}
              >
                {step.message}
              </Typography>
            </StepLabel>
          </Step>
        ))}
      </Stepper>
    </StepperContainer>
  )
}
