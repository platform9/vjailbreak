import { styled, Typography } from "@mui/material"

const StyledHeader = styled("header")(({ theme }) => ({
  borderBottom: `1px solid ${theme.palette.divider}`,
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
