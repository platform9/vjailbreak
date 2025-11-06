package k8sutils

type VjailbreakSettings struct {
	ChangedBlocksCopyIterationThreshold int
	PeriodicSyncInterval                string
	VMActiveWaitIntervalSeconds         int
	VMActiveWaitRetryLimit              int
	DefaultMigrationMethod              string
	VCenterScanConcurrencyLimit         int
	CleanupVolumesAfterConvertFailure   bool
	PopulateVMwareMachineFlavors        bool
	VolumeAvailableWaitIntervalSeconds  int
	VolumeAvailableWaitRetryLimit       int
	VCenterLoginRetryLimit              int
	OpenstackCredsRequeueAfterMinutes   int
	VMwareCredsRequeueAfterMinutes      int
	ValidateRDMOwnerVMs                 bool
	MaxRetries                          int
}
