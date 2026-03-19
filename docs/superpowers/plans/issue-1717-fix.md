# Fix: Orphaned Network shown for DVS port group networks in VMwareMachine CR

**Issue:** #1717 | **Branch:** fix/issue-1717-dvs-portgroup-network-lookup
**Goal:** Fix DVS portgroup network name lookup by preserving MOR type through the NIC struct
**Test command:** `cd k8s/migration && go test ./pkg/utils/... -v -run TestExtractVirtualNICs`

### Task 1: Add NetworkType field to NIC struct
**Files:** `k8s/migration/api/v1alpha1/vmwaremachine_types.go`

- [ ] Add `NetworkType string` field to the `NIC` struct with `json:"networkType,omitempty"`
- [ ] Run `cd k8s/migration && make manifests generate` to regenerate CRDs
- [ ] `git commit -m "feat: add NetworkType field to NIC struct for correct MOR lookup"`

### Task 2: Populate NetworkType in ExtractVirtualNICs
**Files:** `k8s/migration/pkg/utils/credutils.go`

- [ ] Write failing test for ExtractVirtualNICs that verifies DVS portgroup backing sets NetworkType="DistributedVirtualPortgroup"
- [ ] Run: `go test ./pkg/utils/... -v -run TestExtractVirtualNICs` — expect FAIL
- [ ] Update `ExtractVirtualNICs` to set `NetworkType` based on backing type:
  - `VirtualEthernetCardNetworkBackingInfo` → `"Network"`
  - `VirtualEthernetCardDistributedVirtualPortBackingInfo` → `"DistributedVirtualPortgroup"`
  - `VirtualEthernetCardOpaqueNetworkBackingInfo` → `"OpaqueNetwork"`
- [ ] Run: `go test ./pkg/utils/... -v -run TestExtractVirtualNICs` — expect PASS
- [ ] `git commit -m "fix: populate NetworkType in ExtractVirtualNICs for correct MOR lookup"`

### Task 3: Use NetworkType in property collector lookup
**Files:** `k8s/migration/pkg/utils/credutils.go` (line ~1728)

- [ ] Replace hardcoded `Type: "Network"` with `Type: nic.NetworkType` (with fallback to "Network" for backward compat)
- [ ] Run: `go test ./pkg/utils/... -v` — expect PASS
- [ ] `git commit -m "fix: use correct MOR type for network property collector lookup"`
