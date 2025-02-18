import { Drawer, styled } from "@mui/material";

export const StyledDrawer = styled(Drawer)(() => ({
    "& .MuiDrawer-paper": {
        display: "grid",
        gridTemplateRows: "max-content 1fr max-content",
        width: "800px",
    },
}));

export const DrawerContent = styled("div")(({ theme }) => ({
    overflow: "auto",
    padding: theme.spacing(4, 6, 4, 4),
})); 