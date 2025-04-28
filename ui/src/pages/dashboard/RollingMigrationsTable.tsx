import { useState } from "react";
import {
    Box,
    Tab,
    Tabs
} from "@mui/material";
import { ClusterMigration } from "src/api/clustermigrations/model";
import { ESXIMigration } from "src/api/esximigrations/model";
import ClusterMigrationsTable from "./ClusterMigrationsTable";
import ESXIMigrationsTable from "./ESXIMigrationsTable";
import { QueryObserverResult } from "@tanstack/react-query";
import { RefetchOptions } from "@tanstack/react-query";

interface TabPanelProps {
    children?: React.ReactNode;
    index: number;
    value: number;
}

function TabPanel(props: TabPanelProps) {
    const { children, value, index, ...other } = props;

    return (
        <div
            role="tabpanel"
            hidden={value !== index}
            id={`rolling-migration-tabpanel-${index}`}
            aria-labelledby={`rolling-migration-tab-${index}`}
            {...other}
            style={{ height: "calc(100% - 48px)" }}
        >
            {value === index && (
                <Box sx={{ height: "100%" }}>
                    {children}
                </Box>
            )}
        </div>
    );
}

interface RollingMigrationsTableProps {
    clusterMigrations: ClusterMigration[];
    esxiMigrations: ESXIMigration[];
    onDeleteClusterMigration: (name: string) => void;
    onDeleteESXIMigration: (name: string) => void;
    onDeleteSelectedClusterMigrations: (clusterMigrations: ClusterMigration[]) => void;
    onDeleteSelectedESXIMigrations: (esxiMigrations: ESXIMigration[]) => void;
    refetchClusterMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterMigration[], Error>>;
    refetchESXIMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<ESXIMigration[], Error>>;
}

export default function RollingMigrationsTable({
    clusterMigrations,
    esxiMigrations,
    onDeleteClusterMigration,
    onDeleteESXIMigration,
    onDeleteSelectedClusterMigrations,
    onDeleteSelectedESXIMigrations,
    refetchClusterMigrations,
    refetchESXIMigrations
}: RollingMigrationsTableProps) {
    const [activeTab, setActiveTab] = useState(0);

    const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
        setActiveTab(newValue);
    };

    return (
        <Box sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
            <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                <Tabs
                    value={activeTab}
                    onChange={handleTabChange}
                    aria-label="rolling migration tables"
                >
                    <Tab label="Cluster Migrations" />
                    <Tab label="ESXi Migrations" />
                </Tabs>
            </Box>
            <TabPanel value={activeTab} index={0}>
                <ClusterMigrationsTable
                    clusterMigrations={clusterMigrations}
                    onDeleteClusterMigration={onDeleteClusterMigration}
                    onDeleteSelected={onDeleteSelectedClusterMigrations}
                    refetchClusterMigrations={refetchClusterMigrations}
                />
            </TabPanel>
            <TabPanel value={activeTab} index={1}>
                <ESXIMigrationsTable
                    esxiMigrations={esxiMigrations}
                    onDeleteESXIMigration={onDeleteESXIMigration}
                    onDeleteSelected={onDeleteSelectedESXIMigrations}
                    refetchESXIMigrations={refetchESXIMigrations}
                />
            </TabPanel>
        </Box>
    );
} 