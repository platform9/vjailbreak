import { styled, Typography } from "@mui/material"

const StyledHeader = styled("header")(() => ({
  borderBottom: "1px solid #CDD0D4",
}))

interface HeaderProps {
  title: string
}

export default function Header({ title }: HeaderProps) {
  return (
    <StyledHeader>
      <Typography variant="h3" sx={{ padding: 2 }}>
        {title}
      </Typography>
    </StyledHeader>
  )
}
