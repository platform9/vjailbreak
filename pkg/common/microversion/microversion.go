// Package microversion provides an operator-configurable floor over hardcoded
// OpenStack service microversion values.
//
// vjailbreak hardcodes microversion values for specific operations (for example,
// 2.60 on the Nova compute attach call to support multi-attach volumes). When
// the operator configures a per-service API version through clouds.yaml
// (compute_api_version, volume_api_version, image_api_version,
// network_api_version, identity_api_version), the value is consumed as a floor:
// the higher of the operator-configured version and the internal hardcoded
// version wins, so a misconfigured low version cannot lower the microversion
// the operation requires.
package microversion

// Floor returns the higher of two OpenStack microversion strings of the form
// "MAJOR.MINOR". The literal value "latest" is treated as greater than any
// specific version. An empty configValue is treated as "no operator override".
//
// Stub implementation: always returns the empty string so the package tests
// fail until Floor is implemented properly. See [[research.md R-5]] for the
// semantics this function must satisfy.
func Floor(configValue, hardcodedValue string) string {
	_ = configValue
	_ = hardcodedValue
	return ""
}
