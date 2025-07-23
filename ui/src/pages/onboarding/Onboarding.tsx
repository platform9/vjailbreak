import CheckCircleIcon from "@mui/icons-material/CheckCircle"
import KeyIcon from "@mui/icons-material/Key"
import SwapHorizIcon from "@mui/icons-material/SwapHoriz"
import HubIcon from "@mui/icons-material/Hub"
import { Typography, Box, Card, Fade, Slide } from "@mui/material"
import Button from "@mui/material/Button"
import { styled } from "@mui/system"
import { useState } from "react"
import { useNavigate, useLocation } from "react-router-dom"
import MigrationFormDrawer from "../../features/migration/MigrationForm"
import RollingMigrationFormDrawer from "../../features/migration/RollingMigrationForm"
import VMwareCredentialsDrawer from "../../components/drawers/VMwareCredentialsDrawer"
import OpenstackCredentialsDrawer from "../../components/drawers/OpenstackCredentialsDrawer"
import { useVmwareCredentialsQuery } from "../../hooks/api/useVmwareCredentialsQuery"
import { useOpenstackCredentialsQuery } from "../../hooks/api/useOpenstackCredentialsQuery"
import Platform9Logo from "../../components/Platform9Logo"

const OnboardingContainer = styled("div")({
  display: "flex",
  justifyContent: "center",
  alignItems: "flex-start",
  minHeight: "100%",
  overflowY: "auto",
  "&::-webkit-scrollbar": {
    width: "6px",
  },
  "&::-webkit-scrollbar-track": {
    background: "transparent",
  },
  "&::-webkit-scrollbar-thumb": {
    background: "#c1c1c1",
    borderRadius: "3px",
  },
  "&::-webkit-scrollbar-thumb:hover": {
    background: "#a8a8a8",
  },
  scrollbarWidth: "thin",
  scrollbarColor: "#c1c1c1 transparent",
})

const Container = styled("div")(({ theme }) => ({
  display: "grid",
  alignItems: "center",
  justifyItems: "center",
  gap: theme.spacing(4),
  textAlign: "center",
  width: "100%",
  maxWidth: "1200px",
  paddingTop: theme.spacing(2),
  paddingBottom: theme.spacing(4),
}))

const LogoContainer = styled("div")(({ theme }) => ({
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  gap: theme.spacing(2),
  marginBottom: theme.spacing(2),
}))


const StepContainer = styled(Box)(({ theme }) => ({
  display: "flex",
  flexDirection: "column",
  gap: theme.spacing(3),
  width: "100%",
  maxWidth: "800px",
}))

const CredentialCard = styled(Card)(({ theme }) => ({
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  padding: theme.spacing(2),
  marginBottom: theme.spacing(2),
}))


const StepperContainer = styled(Box)(({ theme }) => ({
  width: "100%",
  maxWidth: "800px",
  marginBottom: theme.spacing(4),
}))

const NavigationContainer = styled(Box)(({ theme }) => ({
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  width: "100%",
  marginTop: theme.spacing(4),
  maxWidth: "800px",
}))

export default function Onboarding() {
  const navigate = useNavigate()
  const location = useLocation()
  const [openMigrationForm, setOpenMigrationForm] = useState(false)
  const [openRollingMigrationForm, setOpenRollingMigrationForm] = useState(false)
  const [openVMwareCredentialsDrawer, setOpenVMwareCredentialsDrawer] = useState(false)
  const [openOpenstackCredentialsDrawer, setOpenOpenstackCredentialsDrawer] = useState(false)
  const [currentStep, setCurrentStep] = useState(0)

  const isOnDashboard = location.pathname.startsWith('/dashboard')

  const { data: vmwareCredentials } = useVmwareCredentialsQuery()
  const { data: openstackCredentials } = useOpenstackCredentialsQuery()

  const hasVmwareCredentials = vmwareCredentials && vmwareCredentials.length > 0
  // const hasOpenstackCredentials = openstackCredentials && openstackCredentials.length > 0
  const hasOpenstackCredentials = true
  const hasAllCredentials = hasVmwareCredentials && hasOpenstackCredentials

  const steps = ['Set up Credentials', 'Choose Migration Type']

  const renderCredentialStep = () => (
    <StepContainer>
      <Typography variant="h5" gutterBottom>
        Step 1: Set up Your Credentials
      </Typography>
      <Typography variant="body1" color="text.secondary" gutterBottom>
        Before you can migrate VMs, you need to set up credentials for both your VMware source environment and OpenStack destination.
      </Typography>

      <Box display="grid" gridTemplateColumns="1fr 1fr" gap={3}>
        <CredentialCard>
          <Box display="flex" flexDirection="column" gap={2}>
            <Box display="flex" alignItems="flex-start" gap={2}>
              <KeyIcon
                sx={{ fontSize: 24, color: hasVmwareCredentials ? "success.main" : "primary.main", mt: 0.5 }}
              />
              <Box flex={1}>
                <Typography variant="h6" gutterBottom>VMware Credentials</Typography>
                <Typography variant="body2" color="text.secondary">
                  Required to access your VMware environment
                </Typography>
              </Box>
              {hasVmwareCredentials && (
                <CheckCircleIcon color="success" sx={{ fontSize: 20 }} />
              )}
            </Box>
            {hasVmwareCredentials ? (
              <Box display="flex" alignItems="center" gap={1} justifyContent="center">
                <Typography variant="body2" color="success.main" fontWeight="medium">Configured</Typography>
              </Box>
            ) : (
              <Button
                variant="contained"
                fullWidth
                sx={{
                  backgroundColor: "#000",
                  color: "#fff",
                  "&:hover": { backgroundColor: "#333" },
                  borderRadius: "8px",
                  textTransform: "none",
                  fontWeight: "medium"
                }}
                onClick={() => setOpenVMwareCredentialsDrawer(true)}
              >
                Add VMware Credentials
              </Button>
            )}
          </Box>
        </CredentialCard>

        <CredentialCard>
          <Box display="flex" flexDirection="column" gap={2}>
            <Box display="flex" alignItems="flex-start" gap={2}>
              <KeyIcon
                sx={{ fontSize: 24, color: hasOpenstackCredentials ? "success.main" : "primary.main", mt: 0.5 }}
              />
              <Box flex={1}>
                <Typography variant="h6" gutterBottom>OpenStack Credentials</Typography>
                <Typography variant="body2" color="text.secondary">
                  Required to access your OpenStack destination
                </Typography>
              </Box>
              {hasOpenstackCredentials && (
                <CheckCircleIcon color="success" sx={{ fontSize: 20 }} />
              )}
            </Box>
            {hasOpenstackCredentials ? (
              <Box display="flex" alignItems="center" gap={1} justifyContent="center">
                <Typography variant="body2" color="success.main" fontWeight="medium">Configured</Typography>
              </Box>
            ) : (
              <Button
                variant="contained"
                fullWidth
                sx={{
                  backgroundColor: "#000",
                  color: "#fff",
                  "&:hover": { backgroundColor: "#333" },
                  borderRadius: "8px",
                  textTransform: "none",
                  fontWeight: "medium"
                }}
                onClick={() => setOpenOpenstackCredentialsDrawer(true)}
              >
                Add OpenStack Credentials
              </Button>
            )}
          </Box>
        </CredentialCard>
      </Box>
    </StepContainer>
  )

  const renderMigrationTypeStep = () => (
    <StepContainer>
      <Typography variant="h5" gutterBottom>
        Step 2: Choose Migration Type
      </Typography>
      <Typography variant="body1" color="text.secondary" gutterBottom>
        Select how you want to migrate your virtual machines and configure your migration preferences.
      </Typography>

      <Box display="grid" gridTemplateColumns="1fr 1fr" gap={3}>
        <CredentialCard>
          <Box display="flex" flexDirection="column" gap={2} width="100%">
            <Box display="flex" alignItems="flex-start" gap={2}>
              <SwapHorizIcon
                sx={{ fontSize: 24, color: "primary.main", mt: 0.5 }}
              />
              <Box flex={1}>
                <Typography variant="h6" gutterBottom>Start Migration</Typography>
                <Typography variant="body2" color="text.secondary">
                  Migrate individual virtual machines
                </Typography>
              </Box>
            </Box>
            <Button
              variant="contained"
              fullWidth
              sx={{
                backgroundColor: "#000",
                color: "#fff",
                "&:hover": { backgroundColor: "#333" },
                borderRadius: "8px",
                textTransform: "none",
                fontWeight: "medium"
              }}
              onClick={() => {
                // navigate('/dashboard/migrations')
                setOpenMigrationForm(true)
              }}
            >
              Start Migration
            </Button>
          </Box>
        </CredentialCard>

        <CredentialCard>
          <Box display="flex" flexDirection="column" gap={2} width="100%">
            <Box display="flex" alignItems="flex-start" gap={2}>
              <HubIcon
                sx={{ fontSize: 24, color: "primary.main", mt: 0.5 }}
              />
              <Box flex={1}>
                <Typography variant="h6" gutterBottom>Start Cluster Conversion</Typography>
                <Typography variant="body2" color="text.secondary">
                  Migrate complete clusters or hosts
                </Typography>
              </Box>
            </Box>
            <Button
              variant="contained"
              fullWidth
              sx={{
                backgroundColor: "#000",
                color: "#fff",
                "&:hover": { backgroundColor: "#333" },
                borderRadius: "8px",
                textTransform: "none",
                fontWeight: "medium"
              }}
              onClick={() => {
                console.log('Start Cluster Conversion clicked', {
                  openRollingMigrationForm,
                  isOnDashboard,
                  location: location.pathname,
                  hasAllCredentials
                })
                setOpenRollingMigrationForm(true)
                // navigate('/dashboard/cluster-conversions')
              }}
            >
              Start Cluster Conversion
            </Button>
          </Box>
        </CredentialCard>
      </Box >
    </StepContainer >
  )


  const handleNext = () => {
    if (currentStep === 0 && hasAllCredentials) {
      setCurrentStep(1)
    }
  }

  const handlePrevious = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1)
    }
  }

  return (
      <OnboardingContainer>
        {openMigrationForm && (
          <MigrationFormDrawer open={openMigrationForm} onClose={() => setOpenMigrationForm(false)} />
        )}
        {openRollingMigrationForm && (
          <RollingMigrationFormDrawer open={openRollingMigrationForm} onClose={() => setOpenRollingMigrationForm(false)} />
        )}
        {openVMwareCredentialsDrawer && (
          <VMwareCredentialsDrawer open onClose={() => setOpenVMwareCredentialsDrawer(false)} />
        )}
        {openOpenstackCredentialsDrawer && (
          <OpenstackCredentialsDrawer open onClose={() => setOpenOpenstackCredentialsDrawer(false)} />
        )}

        <Container>
          <LogoContainer>
            <Platform9Logo size="large" />
          </LogoContainer>
          <Typography variant="body1">
            Ready to migrate from VMware to OpenStack? <br />
            Follow the steps below to get started with your migration journey.
          </Typography>

          <StepperContainer>
            <Box display="flex" alignItems="center" gap={1.5} marginBottom={3}>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  padding: 1.5,
                  borderRadius: "12px",
                  border: `2px solid ${hasAllCredentials ? "#4caf50" : currentStep === 0 ? "#2196f3" : "#e0e0e0"}`,
                  backgroundColor: hasAllCredentials ? "rgba(76, 175, 80, 0.05)" : currentStep === 0 ? "rgba(33, 150, 243, 0.05)" : "#f5f5f5",
                  flex: 1,
                  cursor: "pointer",
                  transition: "all 0.3s ease",
                  "&:hover": {
                    transform: "translateY(-2px)",
                    boxShadow: "0 4px 12px rgba(0, 0, 0, 0.15)",
                  }
                }}
                onClick={() => setCurrentStep(0)}
              >
                <Box
                  sx={{
                    width: 20,
                    height: 20,
                    borderRadius: "50%",
                    border: `1px solid ${hasAllCredentials ? "#4caf50" : currentStep === 0 ? "#2196f3" : "#e0e0e0"}`,
                    backgroundColor: hasAllCredentials ? "#4caf50" : currentStep === 0 ? "#2196f3" : "transparent",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    transition: "all 0.3s ease"
                  }}
                >
                  {(hasAllCredentials || currentStep === 0) && <Box sx={{ width: 6, height: 6, borderRadius: "50%", backgroundColor: "#fff" }} />}
                </Box>
                <Box>
                  <Typography variant="body1" fontWeight="medium" color={hasAllCredentials ? "success.main" : currentStep === 0 ? "primary.main" : "text.secondary"}>
                    Set up Credentials
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Configure VMware and OpenStack credentials
                  </Typography>
                </Box>
              </Box>

              <Box sx={{ display: "flex", alignItems: "center", mx: 1 }}>
                <Box
                  sx={{
                    width: 16,
                    height: 16,
                    borderRadius: "50%",
                    backgroundColor: "#e0e0e0",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center"
                  }}
                >
                  <Box sx={{ fontSize: 12, color: "#666" }}>→</Box>
                </Box>
              </Box>

              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  padding: 1.5,
                  borderRadius: "12px",
                  border: `2px solid ${currentStep === 1 ? "#2196f3" : "#e0e0e0"}`,
                  backgroundColor: currentStep === 1 ? "rgba(33, 150, 243, 0.05)" : "#f5f5f5",
                  flex: 1,
                  cursor: hasAllCredentials ? "pointer" : "not-allowed",
                  opacity: hasAllCredentials ? 1 : 0.6,
                  transition: "all 0.3s ease",
                  "&:hover": {
                    transform: hasAllCredentials ? "translateY(-2px)" : "none",
                    boxShadow: hasAllCredentials ? "0 4px 12px rgba(0, 0, 0, 0.15)" : "none",
                  }
                }}
                onClick={() => hasAllCredentials && setCurrentStep(1)}
              >
                <Box
                  sx={{
                    width: 20,
                    height: 20,
                    borderRadius: "50%",
                    border: `1px solid ${currentStep === 1 ? "#2196f3" : "#e0e0e0"}`,
                    backgroundColor: currentStep === 1 ? "#2196f3" : "transparent",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    transition: "all 0.3s ease"
                  }}
                >
                  {currentStep === 1 && <Box sx={{ width: 6, height: 6, borderRadius: "50%", backgroundColor: "#fff" }} />}
                </Box>
                <Box>
                  <Typography variant="body1" fontWeight="medium" color={currentStep === 1 ? "primary.main" : "text.secondary"}>
                    Choose Migration Type
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Select migration preferences and options
                  </Typography>
                </Box>
              </Box>
            </Box>

            <Box sx={{ position: "relative", marginBottom: 2 }}>
              <Box
                sx={{
                  height: 4,
                  backgroundColor: "#e0e0e0",
                  borderRadius: 2,
                  overflow: "hidden"
                }}
              >
                <Box
                  sx={{
                    height: "100%",
                    backgroundColor: "#2196f3",
                    borderRadius: 2,
                    width: `${((currentStep + 1) / steps.length) * 100}%`,
                    transition: "width 0.5s ease"
                  }}
                />
              </Box>
            </Box>
          </StepperContainer>

          <Slide direction="right" in={currentStep === 0} mountOnEnter unmountOnExit timeout={300}>
            <Box>{renderCredentialStep()}</Box>
          </Slide>
          <Slide direction="left" in={hasAllCredentials && currentStep === 1} mountOnEnter unmountOnExit timeout={300}>
            <Box>{renderMigrationTypeStep()}</Box>
          </Slide>

          <NavigationContainer>
            <Fade in={currentStep > 0} timeout={300}>
              <Button
                onClick={handlePrevious}
                disabled={currentStep === 0}
                sx={{ visibility: currentStep === 0 ? 'hidden' : 'visible' }}
              >
                Previous
              </Button>
            </Fade>
            <Typography variant="body2" color="text.secondary">
              Step {currentStep + 1} of {steps.length}
            </Typography>
            <Fade in={currentStep === 0} timeout={300}>
              <Button
                variant="contained"
                onClick={handleNext}
                disabled={!hasAllCredentials}
                sx={{ visibility: currentStep === 0 ? 'visible' : 'hidden' }}
              >
                Continue
              </Button>
            </Fade>
            <Fade in={currentStep === 1} timeout={300}>
              <Box sx={{ visibility: currentStep === 1 ? 'visible' : 'hidden' }}></Box>
            </Fade>
          </NavigationContainer>

        </Container>
      </OnboardingContainer>
    )
}