import { useEffect, useState } from "react";
import { getMigrationLogs } from "src/api/migration-details/migrationDetails";
import {
    Dialog, DialogTitle, DialogContent, CircularProgress
} from "@mui/material";

export function useMigrationLogs(migrationName: string, phase: string) {
    const [logs, setLogs] = useState<string>("");
    const [loading, setLoading] = useState<boolean>(true);

    useEffect(() => {
        let interval: NodeJS.Timeout | null = null;
        let isActive = true;

        async function fetchLogs() {
            try {
                const response = await getMigrationLogs(migrationName);
                if (isActive) {
                    setLogs(response ? String(response) : "");  // Ensures response is a string
                    setLoading(false);
                }
            } catch (err) {
                console.error("Failed to fetch logs", err);
                if (isActive) setLoading(false);
            }
        }

        fetchLogs(); // initial fetch

        const isPolling = phase !== "Succeeded" && phase !== "Failed";

        if (isPolling) {
            interval = setInterval(() => {
                fetchLogs();
            }, 5000); // Poll every 5 seconds
        }

        return () => {
            isActive = false;
            if (interval) clearInterval(interval);
        };
    }, [migrationName, phase]);

    return { logs, loading };
}


export default function MigrationDetailsDialog({ open, onClose, migrationName, phase }) {
    const { logs, loading } = useMigrationLogs(migrationName, phase);

    return (
        <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
            <DialogTitle>Logs for {migrationName}</DialogTitle>
            <DialogContent>
                {loading ? (
                    <CircularProgress />
                ) : (
                    <pre style={{ whiteSpace: "pre-wrap", maxHeight: "500px", overflowY: "auto" }}>
                        {logs || "No logs available."}
                    </pre>
                )}
            </DialogContent>
        </Dialog>
    );
}