package upgrade

import "time"

// UpgradeProgress represents the progress of an upgrade or rollback operation.
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
	JobID            string            `json:"jobId,omitempty"`
	PodName          string            `json:"podName,omitempty"`
	Result           string            `json:"result,omitempty"`
}

const (
	StatusPending            = "pending"
	StatusInProgress         = "in_progress"
	StatusDeploying          = "deploying"
	StatusVerifyingStability = "verifying_stability"
	StatusCompleted          = "completed"
	StatusFailed             = "failed"
	StatusRollingBack        = "rolling_back"
	StatusRolledBack         = "rolled_back"
	StatusRollbackFailed     = "rollback_failed"
	StatusUnknown            = "unknown"
)

// One step per deployment manifest applied, so this grows when a deployment is added.
const TotalUpgradeSteps = 12

const TotalRollbackSteps = 5

// Timing knobs for the upgrade flow. Declared as vars rather than consts only so tests can
// shorten them; treat them as constants everywhere else.
var (
	// How long to wait for a deployment to become ready or to scale to zero, and how often
	// to re-check.
	deploymentWaitTimeout  = 5 * time.Minute
	deploymentPollInterval = 10 * time.Second

	// How long to wait for the credential finalizers to finish removing the resources they
	// own after cleanup, and how often to re-check. The OpenstackCreds finalizer will not
	// complete until its non-master agent nodes are gone, so this is not instant.
	cleanupDrainTimeout      = 3 * time.Minute
	cleanupDrainPollInterval = 5 * time.Second
)
