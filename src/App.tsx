import { styled } from "@mui/material"
import "./App.css"
import "./assets/reset.css"
import Onboarding from "./pages/onboarding/Onboarding"

const AppFrame = styled("div")(({ theme }) => ({
  margin: "0 auto",
  width: "100%",
  height: "100%",
  [theme.breakpoints.up("lg")]: {
    maxWidth: "1600px",
  },
}))

function App() {
  return (
    <AppFrame>
      <Onboarding />
    </AppFrame>
  )
}

export default App
