package utils

type VcenterSettings struct {
	ChangedBlocksCopyIterationThreshold int
	VMActiveWaitIntervalSeconds         int
	VMActiveWaitRetryLimit              int
	DefaultMigrationMethod              string
	VCenterScanConcurrencyLimit         int
}
