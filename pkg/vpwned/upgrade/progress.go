package upgrade

import "time"

// UpgradeProgress represents the progress of an upgrade or rollback operation.
// This type is shared between server and executor to ensure consistent JSON serialization.
type UpgradeProgress struct {
	CurrentStep      string            `json:"currentStep"`
	TotalSteps       int               `json:"totalSteps"`
	CompletedSteps   int               `json:"completedSteps"`
	Status           string            `json:"status"`
	Error            string            `json:"error,omitempty"`
	StartTime        time.Time         `json:"startTime"`
	EndTime          *time.Time        `json:"endTime,omitempty"`
	PreviousVersion  string            `json:"previousVersion"`
	TargetVersion    string            `json:"targetVersion"`
	BackupID         string            `json:"backupId,omitempty"`
	OriginalReplicas map[string]int32  `json:"originalReplicas,omitempty"`
	PhaseTimings     map[string]string `json:"phaseTimings,omitempty"`
	JobID            string            `json:"jobId,omitempty"`   // Job UID for debugging
	PodName          string            `json:"podName,omitempty"` // Pod name for debugging
	Result           string            `json:"result,omitempty"`  // "success" or "failure" for simpler server logic
}

// Progress status constants - simplified for UI consumption
// UI should only show these major states to users
const (
	StatusPending            = "pending"             // Server created job, waiting for pod to start
	StatusInProgress         = "in_progress"         // Job is actively running upgrade
	StatusDeploying          = "deploying"           // Applying manifests and updating deployments
	StatusVerifyingStability = "verifying_stability" // Waiting for deployments to be ready
	StatusCompleted          = "completed"           // Upgrade finished successfully
	StatusFailed             = "failed"              // Upgrade failed (may need rollback)
	StatusRollingBack        = "rolling_back"        // Rollback in progress
	StatusRolledBack         = "rolled_back"         // Rollback completed
	StatusRollbackFailed     = "rollback_failed"     // Rollback failed
	StatusUnknown            = "unknown"             // State cannot be determined
)

// TotalUpgradeSteps is the accurate count of upgrade phases:
// 1. Precheck, 2. Backup, 3. CRD, 4. ConfigMap, 5. Controller scale down,
// 6. Controller apply, 7. Controller scale up, 8. VPWNED apply,
// 9. UI apply, 10. Stability verify, 11. Cleanup
const TotalUpgradeSteps = 11

// TotalRollbackSteps is the count of rollback phases
const TotalRollbackSteps = 5
