import { useCallback, useEffect, useMemo, useState } from "react"
import {
  Stepper,
  Step,
  StepLabel,
  CircularProgress,
  styled,
  StepConnector,
  stepConnectorClasses,
  Popover,
  Typography,
  Box,
} from "@mui/material"
import CheckCircleOutlineIcon from "@mui/icons-material/CheckCircleOutline"
import WarningAmberIcon from "@mui/icons-material/WarningAmber"

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
  message: String
  reason?: String
  lastTransitionTime?: String
}

const defaultSteps: Object = {
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

//Styles
const ColorlibConnector = styled(StepConnector)(({ theme }) => ({
  [`&.${stepConnectorClasses.alternativeLabel}`]: {
    top: 22,
  },
  [`&.${stepConnectorClasses.active}`]: {
    [`& .${stepConnectorClasses.line}`]: {
      backgroundColor: "green",
    },
  },
  [`&.${stepConnectorClasses.completed}`]: {
    [`& .${stepConnectorClasses.line}`]: {
      backgroundColor: "green",
    },
  },
  [`& .${stepConnectorClasses.line}`]: {
    height: 3,
    border: 0,
    backgroundColor: "#eaeaf0",
    borderRadius: 1,
    ...theme.applyStyles("dark", {
      backgroundColor: theme.palette.grey[800],
    }),
  },
}))

const StepperContainer = styled("div")({
  width: "100%",
})

export default function MigrationProgress({ keyLabel, conditions }) {
  const [steps, setSteps] = useState<StatusStep[]>(Object.values(defaultSteps))

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
  }, [conditions])

  const activeStep = useMemo(() => {
    return steps.findIndex((step) => step.status === Status.InProgress)
  }, [steps])

  return (
    <Box height={52} display={"flex"} alignItems={"center"}>
      <StepperContainer key={keyLabel}>
        <Stepper activeStep={activeStep} connector={<ColorlibConnector />}>
          {steps.map((step, index) => (
            <Step
              key={`${index}+${String(step.type)}`}
              completed={[Status.Completed, Status.Failed].includes(
                step.status
              )}
            >
              <StepContent step={step} />
            </Step>
          ))}
        </Stepper>
      </StepperContainer>
    </Box>
  )
}

const StepContent = ({ step }: { step: StatusStep }) => {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null)
  const handlePopoverOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget)
  }
  const handlePopoverClose = () => {
    setAnchorEl(null)
  }

  // Return step icon based on the status
  const getStepIcon = useCallback((status: Status) => {
    let stepIcon
    switch (status) {
      case Status.Completed:
        stepIcon = () => <CheckCircleOutlineIcon style={{ color: "green" }} />
        break
      case Status.Pending:
        stepIcon = () => <CheckCircleOutlineIcon style={{ color: "grey" }} />
        break
      case Status.InProgress:
        stepIcon = () => <CircularProgress size={20} sx={{ color: "green" }} />
        break
      case Status.Failed:
        stepIcon = () => <WarningAmberIcon style={{ color: "red" }} />
        break
      default:
        break
    }
    return stepIcon
  }, [])

  const lastUpdated = step?.lastTransitionTime
    ? new Date(String(step?.lastTransitionTime)).toTimeString()
    : null

  return (
    <>
      <StepLabel
        StepIconComponent={getStepIcon(step.status)}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
      >
        <Typography
          sx={{ color: step.status === Status.Failed ? "red" : "default" }}
        >
          {step.message}
        </Typography>
      </StepLabel>
      {step.status !== Status.Pending && (
        <Popover
          id="mouse-over-popover"
          sx={{ pointerEvents: "none" }}
          open={Boolean(anchorEl)}
          anchorEl={anchorEl}
          anchorOrigin={{
            vertical: "bottom",
            horizontal: "left",
          }}
          transformOrigin={{
            vertical: "top",
            horizontal: "left",
          }}
          onClose={handlePopoverClose}
          disableRestoreFocus
        >
          <Typography sx={{ p: 1 }}>{step.message}</Typography>
          {lastUpdated && (
            <Typography sx={{ p: 1 }}>Last Updated: {lastUpdated}</Typography>
          )}
        </Popover>
      )}
    </>
  )
}
