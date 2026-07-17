# Quickstart: Migration Templates and Saved Configurations

## Backend: extend the CRD

```bash
cd k8s/migration
# edit api/v1alpha1/migrationtemplate_types.go per plan.md's "Implementation Notes"
make generate      # regenerates zz_generated.deepcopy.go + config/crd/bases/*migrationtemplates*.yaml
make test          # controller unit tests
```

From the repo root, refresh the generated manifests (requires `vjail-controller` and `ui` built first, per CLAUDE.md):

```bash
make vjail-controller ui
make generate-manifests   # regenerates deploy/00crds.yaml and deploy/installer.yaml — never hand-edit these
```

Verify no RBAC diff appears for `ui-manager-role` (it already grants `migrationtemplates/status` get/patch/update — see research.md Decision 2). If a diff *does* appear, stop and investigate before committing — that would mean an assumption in this plan was wrong.

## Frontend: local dev

```bash
cd ui
export VITE_API_HOST=<backend-host>
export VITE_API_TOKEN=<auth-token>
yarn dev
```

## Manual verification checklist (maps to spec.md User Stories)

1. **Save (US1)**: Open New Migration, fill source/destination + at least one mapping, click "Save as template", name it, confirm it appears under the new "Templates" tab on `/dashboard/migrations`.
2. **Browse (US2)**: On the Templates tab, search by the name you just used, sort by "Last used" — confirm the card you saved is findable and its tags/summary match what you entered.
3. **Apply (US3)**: Click "Use" on that card — confirm the New Migration drawer opens with source/destination/mappings/options pre-filled and every field still editable. Submit the migration; re-open the Templates tab and confirm "Times Used" incremented and "Last Used" updated on that template's detail drawer.
4. **Stale reference (US3 edge case)**: Delete the network/storage mapping referenced by a saved template, then click "Use" on it again — confirm the mapping field is left blank with an inline warning, not a crash or silent wrong value.
5. **Detail view (US4)**: Click a card (not "Use") — confirm the `DrawerShell`-based detail drawer opens over a dimmed list, shows Times Used/Last Used/Created plus full Source & Destination and Network & Storage Mappings sections, and closes cleanly via the X or backdrop click.
6. **Delete/Clone (US5)**: Clone a template, confirm the clone appears with a "(copy)" suffix and independent identity; delete the original — confirm any migration previously submitted from it is unaffected (still visible/runnable under the existing Migrations tab); delete the clone too and confirm it disappears from the Templates list.
7. **Regression — ephemeral lifecycle unaffected**: Open a *fresh* New Migration drawer (not via "Use"), fill in creds, and click Cancel — confirm (via `kubectl -n migration-system get migrationtemplates`) that the auto-created ephemeral (uuid-named, `saved` unset) template is deleted as it always was, and that no saved template was touched.
8. **Regression — retry flow unaffected**: Retry a previously failed/completed migration (not created from a saved template) and confirm the existing retry-prefill behavior is unchanged.
