# Data Model: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Feature**: [spec.md](./spec.md) | **Date**: 2026-07-06

No new CRDs. One new optional field on an existing CRD, one new context key convention, and two new Go interfaces (see [contracts/interfaces.md](./contracts/interfaces.md)).

## ArrayCreds.spec.mappingMode (new field)

| Property | Value |
|---|---|
| CRD | `arraycreds.vjailbreak.k8s.pf9.io/v1alpha1` |
| Path | `spec.mappingMode` |
| Type | string |
| Validation | `+kubebuilder:validation:Enum=auto;native;cinder`, `+optional` |
| Empty value | Treated as `auto` everywhere (selector and reconciler) |
| Go constants | `MappingModeAuto = "auto"`, `MappingModeNative = "native"`, `MappingModeCinder = "cinder"` (api/v1alpha1) |

### Semantics (selector truth table)

| mappingMode | Provider satisfies VendorMapper? | Mapper chosen | Where enforced |
|---|---|---|---|
| "" / auto | yes | vendor-native (adapter) | v2v-helper selectMapper |
| "" / auto | no | CinderMapper | v2v-helper selectMapper |
| native | yes | vendor-native (adapter) | v2v-helper selectMapper |
| native | no | — error | ArrayCreds reconciler (CR → Failed) **and** selectMapper (defense in depth) |
| cinder | any | CinderMapper | v2v-helper selectMapper |
| anything else | — | — rejected | API server admission (CRD enum) |

### Status effects

`mappingMode: native` + vendor without `VendorMapper` ⇒ `status.phase: Failed`, `status.arrayValidationStatus: Failed`, `status.arrayValidationMessage: "MappingMode=native unsupported by vendor <type>"`. All other combinations leave the existing validation flow unchanged.

## MappingContext connector entry (Cinder path only)

`storage.MappingContext` is the existing `map[string]interface{}`. The Cinder path stores exactly one key:

| Key | Type | Producer | Consumers |
|---|---|---|---|
| `connector` | `map[string]any` | `CinderMapper.CreateOrUpdateInitiatorGroup` | `CinderMapper.MapVolumeToGroup`, `CinderMapper.UnmapVolumeFromGroup` |

The same connector value MUST be presented to both `os-initialize_connection` and `os-terminate_connection` — drivers use it to identify which export to tear down. Vendor-native paths keep their existing provider-specific keys (Pure: `hosts`; NetApp: igroup data); key spaces do not overlap because a MappingContext never crosses mapper implementations.

### Connector dict schema (os-brick-shaped)

| Key | Type | Present | Value |
|---|---|---|---|
| `host` | string | always | `vjailbreak-<sanitized ESXi IP>`; fallback `vjailbreak-xcopy` |
| `ip` | string | when ESXi IP known | ESXi management IP |
| `platform` | string | always | `x86_64` |
| `os_type` | string | always | `linux` |
| `multipath` | bool | always | `true` |
| `initiator` | string | ≥1 iSCSI adapter | first IQN (lowercase, as reported by ESXi) |
| `wwpns` | []string | ≥1 FC adapter | lowercase colon-stripped hex WWPNs |
| `wwnns` | []string | ≥1 FC adapter | lowercase colon-stripped hex WWNNs |

Input: the `[]string` from `esxiClient.GetAllHostAdapters()` — entries are lowercase `iqn....` or `fc.<WWNN>:<WWPN>`. Malformed `fc.*` entries are skipped with a warning; zero usable initiators is an error.

## Volume flow (unchanged, annotated for the Cinder path)

```
vendor CreateVolume ──► Volume{Name, Size, NAA}                 (vendor REST; NAA from serial)
        │
        ▼
manageVolumeToCinder ──► cinderVolumeID                          (Cinder manage; driver renames LUN)
        │
        ▼
ResolveCinderVolumeToLUN(cinderVolumeID) ──► renamed LUN name    (vendor REST)
        │
        ▼
targetVol = Volume{Name: renamed, NAA, Size, OpenstackVol.ID: cinderVolumeID}
        │
        ▼
mapper.MapVolumeToGroup ──► native: array host connect           (vendor REST)
        │                   cinder: os-initialize_connection(OpenstackVol.ID, connector)
        ▼
ESXi rescan by NAA ──► vmkfstools XCOPY ──► deferred mapper.UnmapVolumeFromGroup
```

Invariant: `Volume.OpenstackVol.ID` MUST be non-empty before `MapVolumeToGroup` on the Cinder path (guaranteed by ordering: manage precedes map; `CinderMapper` errors defensively if violated).

## State & lifecycle notes

- `os-initialize_connection` creates **no** Cinder attachment record; the volume remains `available` throughout the copy window. Observability is array-side or via cinder-volume logs plus the v2v-helper connector log line.
- The export's lifecycle is bounded by `copyDiskViaStorageAcceleratedCopy` (map → clone → deferred unmap with fresh timeout ctx). A pod crash inside the window leaks the export (documented; parity with native path).
