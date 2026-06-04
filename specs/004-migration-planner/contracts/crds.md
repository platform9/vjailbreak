# Contracts: MigrationBucket CRD

**Group/Version**: `vjailbreak.k8s.pf9.io/v1alpha1`
**Kind**: `MigrationBucket` (namespaced, namespace `migration-system`)
**Source**: `k8s/migration/api/v1alpha1/migrationbucket_types.go`

## Schema (summary)

```
spec:
  vmwareCredsRef: { name: string }      # source VMware credential (required)
  vms: [string]                          # member VM names (required, ≥1)
  isDefault: bool                        # the auto-created default bucket (non-deletable)
  schedule: <metav1.Time>                # optional future start time
  config:
    sourceCluster: string
    pcdCluster: string
    networkMappings: [{ source, target }]
    storageMappings: [{ source, target }]
    securityGroups: [string]
    serverGroup: string
    dataCopyMethod: string
    formValues: <RawExtension>           # full Migration Form inputs (round-trip), opaque
    selectedOptions: <RawExtension>      # which option checkboxes were enabled, opaque
status:
  phase: NotMigrated | Scheduled | InProgress | Migrated
  message: string                        # e.g. invariant violation detail
```

`formValues` / `selectedOptions` use `+kubebuilder:pruning:PreserveUnknownFields` so the editor
can store and round-trip the exact Migration Form state without the CRD enumerating every field.

## REST surface (served by kube-apiserver, proxied to the UI)

- `GET    /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationbuckets`
- `POST   …/migrationbuckets`
- `PUT    …/migrationbuckets/{name}`
- `DELETE …/migrationbuckets/{name}`

The UI client (`ui/src/features/inventory/api/migration-buckets/migrationBuckets.ts`) targets
exactly these with `BUCKETS_DATA_SOURCE = 'api'`.

## Controller

`internal/controller/migrationbucket_controller.go` — `MigrationBucketReconciler` defaults the
status phase and surfaces the no-empty-bucket invariant in `status.message`. It does **not**
modify the existing Migration/MigrationPlan workflow.

## Build & deploy (required — run locally; Go toolchain needed)

```bash
cd k8s/migration
make generate        # regenerate zz_generated.deepcopy.go (DO NOT hand-edit)
make manifests       # regenerate CRD YAML + RBAC role from kubebuilder markers
make test            # runs the new reconciler unit tests
# then rebuild/redeploy the controller image and apply CRDs:
make build-image     # or your usual controller build/deploy
kubectl apply -f config/crd/bases/vjailbreak.k8s.pf9.io_migrationbuckets.yaml
```

`make generate-manifests` (repo root) regenerates `deploy/installer.yaml` / `deploy/00crds.yaml`
— do not hand-edit those.

## Not yet implemented (follow-ups)

- **Trigger → compile** (Phase 9, T042–T046): turning selected buckets into `MigrationPlan` +
  `RollingMigrationPlan` and scaling `VjailbreakNode` workers. The UI trigger dialog currently
  stubs the confirm action.
- **Backend default-bucket creation** (Q2): currently the UI creates the default bucket via the
  API on first visit; moving creation into a reconciler/watch is a follow-up.
- **Validating webhook** for hard enforcement of the invariants (currently UI-enforced + surfaced
  in status).
