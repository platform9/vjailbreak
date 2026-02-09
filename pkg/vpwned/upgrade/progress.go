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

// Progress status constants
const (
	StatusInProgress               = "in_progress"
	StatusDeploying                = "deploying"
	StatusCompleted                = "completed"
	StatusFailed                   = "failed"
	StatusRollingBack              = "rolling_back"
	StatusRolledBack               = "rolled_back"
	StatusRollbackFailed           = "rollback_failed"
	StatusDeploymentsReadyUnstable = "deployments_ready_but_unstable"
	StatusUnknown                  = "unknown"
	StatusVerifyingStability       = "verifying_stability"
)

// TotalUpgradeSteps is the accurate count of upgrade phases:
// 1. Precheck, 2. Backup, 3. CRD, 4. ConfigMap, 5. Controller scale down,
// 6. Controller apply, 7. Controller scale up, 8. VPWNED apply,
// 9. UI apply, 10. Stability verify, 11. Cleanup
const TotalUpgradeSteps = 11

// TotalRollbackSteps is the count of rollback phases
const TotalRollbackSteps = 5
