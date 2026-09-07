package catalog

// VM is the subset of per-VM data that the tier-1 ("vCenter creds only")
// pre-check rules need. It mirrors fields reachable from govmomi's
// mo.VirtualMachine without requiring VMware Tools, a guest script, or a
// destination (PCD) credential — see the PRD's data-coverage ladder
// (vCenter creds only -> +Tools -> +guest script -> +PCD).
//
// The discovery engine (#2387) is what will actually populate this struct
// from a live vCenter; every rule in this package is a pure function of VM,
// so it is fully unit-testable today with fixture data.
type VM struct {
	Name string

	// Template mirrors config.template.
	Template bool

	// PowerState mirrors runtime.powerState ("poweredOn" or "poweredOff").
	PowerState string

	// HWVersion is the numeric virtual hardware version (e.g. 15 for vmx-15).
	HWVersion int

	// CBTEnabled mirrors config.changeTrackingEnabled. nil means unknown
	// (the field was unset), which the CBT rule treats the same as false.
	CBTEnabled *bool

	// Annotation mirrors config.annotation.
	Annotation string

	// VTPMPresent is true if a VirtualTPM device is present in
	// config.hardware.device.
	VTPMPresent bool

	// EncryptionKeyID mirrors config.keyId; non-empty means the VM (or a
	// disk) is encrypted.
	EncryptionKeyID string

	// GuestOSID is the configured guest OS short id (config.guestId), e.g.
	// "rhel8_64Guest".
	GuestOSID string

	// NICIPv4s lists every IPv4 address seen across all NICs, for checks
	// like APIPA detection. Populating this from vCenter creds alone
	// requires VMware Tools in practice; it is included here because the
	// check itself is a pure function of the address list.
	NICIPv4s []string
}
