import { useEffect } from "react"
import { ClusterMigration } from "src/api/clustermigrations/model"
import { useAmplitude } from "./useAmplitude"
import { useErrorHandler } from "./useErrorHandler"
import { useStatusTracker } from "./useStatusMonitor"
import { AMPLITUDE_EVENTS } from "src/types/amplitude"

export const useClusterMigrationStatusMonitor = (
  clusterMigrations: ClusterMigration[] = []
) => {
  const { track } = useAmplitude({ component: "ClusterMigrationStatusMonitor" })
  const { reportError } = useErrorHandler({
    component: "ClusterMigrationStatusMonitor",
  })
  const { statusTrackerRef, autoCleanup } = useStatusTracker<string>()

  useEffect(() => {
    if (!clusterMigrations || clusterMigrations.length === 0) return

    // Auto-cleanup old trackers
    autoCleanup(clusterMigrations.map(m => m.metadata?.name))

    clusterMigrations.forEach((clusterMigration) => {
      const clusterMigrationName = clusterMigration.metadata?.name
      if (!clusterMigrationName) return

      const currentPhase = clusterMigration.status?.phase
      const tracker = statusTrackerRef.current[clusterMigrationName]

      // Initialize tracker for new cluster migrations
      if (!tracker) {
        statusTrackerRef.current[clusterMigrationName] = {
          previousPhase: currentPhase,
        }
        return
      }

      // Skip if phase hasn't changed or already reported
      if (
        tracker.previousPhase === currentPhase ||
        tracker.lastReportedPhase === currentPhase
      ) {
        return
      }

      // Get error details from status
      const getErrorDetails = () => {
        return {
          message:
            clusterMigration.status?.message ||
            `Cluster Migration ${currentPhase}`,
          phase: currentPhase,
        }
      }

      // Handle cluster migration execution failures
      const isFailed = currentPhase === "Failed"
      if (isFailed && tracker.lastReportedPhase !== currentPhase) {
        const errorDetails = getErrorDetails()

        // Track with Amplitude
        track(AMPLITUDE_EVENTS.CLUSTER_CONVERSION_EXECUTION_FAILED, {
          clusterMigrationName,
          clusterName: clusterMigration.spec?.clusterName,
          currentESXi: clusterMigration.status?.currentESXi,
          previousPhase: tracker.previousPhase,
          currentPhase,
          errorMessage: errorDetails.message,
          vmwareCredsRef: clusterMigration.spec?.vmwareCredsRef?.name,
          openstackCredsRef: clusterMigration.spec?.openstackCredsRef?.name,
          rollingMigrationPlanRef:
            clusterMigration.spec?.rollingMigrationPlanRef?.name,
          esxiSequenceLength:
            clusterMigration.spec?.esxiMigrationSequence?.length || 0,
          namespace: clusterMigration.metadata?.namespace,
        })

        // Report to Bugsnag
        const bugsnagError = new Error(
          `Cluster migration execution failed: ${errorDetails.message}`
        )
        reportError(bugsnagError, {
          context: "cluster-migration-execution-failure",
          metadata: {
            clusterMigrationName,
            clusterName: clusterMigration.spec?.clusterName,
            currentESXi: clusterMigration.status?.currentESXi,
            previousPhase: tracker.previousPhase,
            currentPhase,
            errorMessage: errorDetails.message,
            vmwareCredsRef: clusterMigration.spec?.vmwareCredsRef?.name,
            openstackCredsRef: clusterMigration.spec?.openstackCredsRef?.name,
            rollingMigrationPlanRef:
              clusterMigration.spec?.rollingMigrationPlanRef?.name,
            esxiSequenceLength:
              clusterMigration.spec?.esxiMigrationSequence?.length || 0,
            namespace: clusterMigration.metadata?.namespace,
            fullStatus: clusterMigration.status,
            action: "cluster-migration-execution-failed",
          },
        })

        console.error("Cluster migration execution failed:", {
          clusterMigrationName,
          errorDetails,
          clusterMigration,
        })

        // Mark as reported
        statusTrackerRef.current[clusterMigrationName].lastReportedPhase =
          currentPhase
      }

      // Handle cluster migration success (optional - for analytics)
      const isSucceeded = currentPhase === "Succeeded"
      if (isSucceeded && tracker.lastReportedPhase !== currentPhase) {
        track(AMPLITUDE_EVENTS.CLUSTER_CONVERSION_SUCCEEDED, {
          clusterMigrationName,
          clusterName: clusterMigration.spec?.clusterName,
          currentESXi: clusterMigration.status?.currentESXi,
          previousPhase: tracker.previousPhase,
          currentPhase,
          vmwareCredsRef: clusterMigration.spec?.vmwareCredsRef?.name,
          openstackCredsRef: clusterMigration.spec?.openstackCredsRef?.name,
          rollingMigrationPlanRef:
            clusterMigration.spec?.rollingMigrationPlanRef?.name,
          esxiSequenceLength:
            clusterMigration.spec?.esxiMigrationSequence?.length || 0,
          namespace: clusterMigration.metadata?.namespace,
        })

        // Mark as reported
        statusTrackerRef.current[clusterMigrationName].lastReportedPhase =
          currentPhase
      }

      // Update previous phase
      statusTrackerRef.current[clusterMigrationName].previousPhase =
        currentPhase
    })
  }, [clusterMigrations, track, reportError, autoCleanup])

  return {}
}
