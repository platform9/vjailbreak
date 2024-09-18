import RocketIcon from "@mui/icons-material/Rocket"
import TextSnippetIcon from "@mui/icons-material/TextSnippet"
import { Divider, Link, Typography } from "@mui/material"
import Button from "@mui/material/Button"
import { styled } from "@mui/system"

const Container = styled("div")(({ theme }) => ({
  display: "grid",
  alignItems: "center",
  justifyItems: "center",
  gap: theme.spacing(4),
}))

const CircularLogoContainer = styled("div")(() => ({
  width: "150px",
  height: "150px",
  borderRadius: "50%",
  overflow: "hidden",
  display: "flex",
  justifyContent: "center",
  alignItems: "center",
}))

const InfoSection = styled("div")(({ theme }) => ({
  display: "grid",
  gridTemplateColumns: "1fr 1fr",
  gap: theme.spacing(6),
  borderTop: `1px solid ${theme.palette.grey[500]}`,
  padding: theme.spacing(4),
  justifyItems: "center",
}))

const GuidesSection = styled("div")(({ theme }) => ({
  display: "grid",
  gap: theme.spacing(1),
  gridAutoRows: "min-content",
  padding: theme.spacing(2),
}))

const GuidesSectionHeader = styled("div")(({ theme }) => ({
  display: "grid",
  gridTemplateColumns: "repeat(2, max-content)",
  gap: theme.spacing(2),
}))

const UnorderedList = styled("div")(({ theme }) => ({
  listStyle: "none",
  padding: 0,
  marginTop: theme.spacing(2),
  textAlign: "left",
  "& li": {
    marginBottom: theme.spacing(2),
  },
}))

export default function Onboarding() {
  return (
    <Container>
      <CircularLogoContainer>
        <img
          src="/logo.png"
          alt="vJailbreak Logo"
          css={{ width: "150px", height: "150px", objectFit: "cover" }}
        />
      </CircularLogoContainer>
      <Typography variant="h3">vJailbreak</Typography>
      <Typography variant="subtitle1">
        Ready to migrate from VMware to OpenStack? <br />
        Click below to start moving your VMware workloads to OpenStack with
        ease.
      </Typography>
      <Button variant="contained" size="large">
        Start Migration
      </Button>
      <Divider />
      <InfoSection>
        <GuidesSection>
          <GuidesSectionHeader>
            <RocketIcon />
            <Typography variant="h6">Getting Started</Typography>
          </GuidesSectionHeader>
          <UnorderedList>
            <li>
              <Link href="#" variant="body1">
                Quick Start guide to vJailbreak
              </Link>
            </li>
            <li>
              <Link href="#" variant="body1">
                Supported VMware and Destination Platforms
              </Link>
            </li>
            <li>
              <Link href="#" variant="body1">
                How is Data Copied?
              </Link>
            </li>
            <li>
              <Link href="#" variant="body1">
                How is the VM converted?
              </Link>
            </li>
          </UnorderedList>
        </GuidesSection>
        <GuidesSection>
          <GuidesSectionHeader>
            <TextSnippetIcon />
            <Typography variant="h6">Advanced Topics</Typography>
          </GuidesSectionHeader>
          <UnorderedList>
            <li>
              <Link href="#" variant="body1">
                Migrating Workflow: by VM, by Host, by Cluster
              </Link>
            </li>
            <li>
              <Link href="#" variant="body1">
                Pre and Post Migration Callouts
              </Link>
            </li>
            <li>
              <Link href="#" variant="body1">
                Freeing up ESXi host hardware for KVM
              </Link>
            </li>
          </UnorderedList>
        </GuidesSection>
      </InfoSection>
    </Container>
  )
}
