import { Dialog, DialogTitle, DialogContent, DialogContentText, DialogActions, Button, CircularProgress } from "@mui/material";

interface DeleteDialogProps {
    open: boolean;
    migrationName: string | null;
    onClose: () => void;
    onConfirm: () => void;
    isDeleting: boolean;
    error: string | null;
}

export default function DeleteConfirmationDialog({ open, migrationName, onClose, onConfirm, isDeleting, error }: DeleteDialogProps) {
    return (
        <Dialog
            open={open}
            onClose={onClose}
            PaperProps={{
                sx: {
                    width: '100%',
                    maxWidth: '500px',
                    m: 4,
                    p: 1
                }
            }}
        >
            <DialogTitle sx={{ pb: 2 }}>Confirm Delete</DialogTitle>
            <DialogContent sx={{ px: 3 }}>
                <DialogContentText>
                    Are you sure you want to delete migration "<strong>{migrationName}</strong>"?
                    <br />
                    This action cannot be undone.
                </DialogContentText>
                {error && (
                    <DialogContentText sx={{
                        color: 'error.main',
                        mt: 2
                    }}>
                        Error: {error}
                    </DialogContentText>
                )}
            </DialogContent>
            <DialogActions>
                <Button
                    onClick={onClose}
                    variant="text"
                    sx={{ mr: 1 }}
                    disabled={isDeleting}
                >
                    Cancel
                </Button>
                <Button
                    onClick={onConfirm}
                    color="error"
                    variant="outlined"
                    disabled={isDeleting}
                    sx={{ minWidth: 80 }}
                >
                    {isDeleting ? (
                        <CircularProgress size={20} color="error" />
                    ) : (
                        'Delete'
                    )}
                </Button>
            </DialogActions>
        </Dialog>
    );
}
