import { styled, Typography } from "@mui/material"

const StyledBox = styled("div")(({ theme }) => ({
  display: "flex",
  gap: theme.spacing(2),
  alignItems: "center",
  marginBottom: theme.spacing(2),
}))

const StepCircle = styled("div")(({ theme }) => ({
  display: "flex",
  justifyContent: "center",
  alignItems: "center",
  width: "25px",
  height: "25px",
  borderRadius: "50%",
  fontSize: "16px",
  backgroundColor: theme.palette.grey[200],
  border: `1px solid ${theme.palette.grey[400]}`,
}))

interface StepProps {
  stepNumber: string
  label: string
}

export default function Step({ stepNumber, label }: StepProps) {
  return (
    <StyledBox>
      <StepCircle>
        <Typography variant="body1">{stepNumber}</Typography>
      </StepCircle>
      <Typography variant="subtitle2">{label}</Typography>
    </StyledBox>
  )
}
