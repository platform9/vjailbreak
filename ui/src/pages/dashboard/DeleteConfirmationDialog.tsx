import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogContentText,
    DialogActions,
    Button,
    CircularProgress,
    Tooltip,
    Box,
} from "@mui/material"
import { Migration } from "src/api/migrations/model"

interface DeleteDialogProps {
    open: boolean;
    migrationName: string | null;
    selectedMigrations?: Migration[];
    onClose: () => void;
    onConfirm: () => void;
    isDeleting: boolean;
    error: string | null;
}

const MAX_DISPLAYED_NAMES = 3;

export default function DeleteConfirmationDialog({
    open,
    migrationName,
    selectedMigrations,
    onClose,
    onConfirm,
    isDeleting,
    error
}: DeleteDialogProps) {
    const isBulkDelete = selectedMigrations && selectedMigrations.length > 0;
    const migrationNames = selectedMigrations?.map(m => m.metadata.name) || [];

    const displayedNames = migrationNames.slice(0, MAX_DISPLAYED_NAMES);
    const remainingCount = migrationNames.length - MAX_DISPLAYED_NAMES;

    return (
        <Dialog
            open={open}
            onClose={onClose}
            PaperProps={{
                sx: {
                    width: '100%',
                    maxWidth: '500px',
                    m: 4,
                }
            }}
        >
            <DialogTitle
                sx={{
                    bgcolor: 'white',
                    color: 'black',
                    px: 3,
                    py: 2,
                }}
            >
                Confirm Delete
            </DialogTitle>
            <DialogContent sx={{ px: 3, py: 2.5 }}>
                <DialogContentText>
                    {isBulkDelete ? (
                        <>
                            Are you sure you want to delete these migrations?
                            <Box sx={{ mt: 1 }}>
                                {displayedNames.map((name) => (
                                    <div key={name}>
                                        <strong>{name}</strong>
                                    </div>
                                ))}
                                {remainingCount > 0 && (
                                    <Tooltip title={migrationNames.slice(MAX_DISPLAYED_NAMES).join('\n')}>
                                        <div>and <strong>{remainingCount}</strong> more...</div>
                                    </Tooltip>
                                )}
                            </Box>
                        </>
                    ) : (
                        <>
                            Are you sure you want to delete migration "<strong>{migrationName}</strong>"?
                            <br />
                        </>
                    )}
                </DialogContentText>
                {error && (
                    <DialogContentText sx={{ color: 'error.main', mt: 2 }}>
                        Error: {error}
                    </DialogContentText>
                )}
            </DialogContent>
            <DialogActions sx={{ px: 3, pb: 2 }}>
                <Button
                    onClick={onClose}
                    variant="text"
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
