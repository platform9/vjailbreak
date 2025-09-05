package k8sutils

type VjailbreakSettings struct {
	ChangedBlocksCopyIterationThreshold int
	VMActiveWaitIntervalSeconds         int
	VMActiveWaitRetryLimit              int
	DefaultMigrationMethod              string
	VCenterScanConcurrencyLimit         int
	CleanupVolumesAfterConvertFailure   bool
	PopulateVMwareMachineFlavors        bool
	VCenterLoginRetryLimit              int
}
