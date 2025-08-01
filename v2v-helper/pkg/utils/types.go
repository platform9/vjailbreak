package utils

type VjailbreakSettings struct {
	ChangedBlocksCopyIterationThreshold int
	VMActiveWaitIntervalSeconds         int
	VMActiveWaitRetryLimit              int
	DefaultMigrationMethod              string
	VCenterScanConcurrencyLimit         int
}
