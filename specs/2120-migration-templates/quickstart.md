# Quickstart: Migration Templates and Saved Configurations

> **Updated 2026-07-20**: the backend CRD (`MigrationBlueprint`) already shipped in
> [platform9/vjailbreak#2158](https://github.com/platform9/vjailbreak/pull/2158) — there is nothing to
> extend/generate on the controller side for this feature. If `kubectl get crd migrationblueprints.vjailbreak.k8s.pf9.io`
> 404s on whatever cluster/appliance you're pointed at, that PR (or its equivalent in your checkout)
> hasn't landed there yet — pull it in before expecting the Templates tab to work against a real
> backend. The section below (backend codegen) describes the *original* plan and no longer applies;
> see `data-model.md`/`contracts/crds.md` for the CRD as it actually exists.

## Backend: already shipped, nothing to generate for this feature

```bash
# Sanity check only — confirms the CRD this feature depends on is actually installed:
kubectl get crd migrationblueprints.vjailbreak.k8s.pf9.io
```

## Frontend: local dev

```bash
cd ui
export VITE_API_HOST=<backend-host>
export VITE_API_TOKEN=<auth-token>
yarn dev
```

## Manual verification checklist (maps to spec.md User Stories)

1. **Save (US1)**: Open New Migration, fill source/destination + at least one mapping, click "Save as template", name it, confirm it appears under the "Templates" tab on `/dashboard/migrations`.
2. **Browse (US2)**: On the Templates tab, search by the name you just used, try both grid and list (table) view, sort by "Newest" and "Name" — confirm the card/row you saved is findable and its tags/summary match what you entered. (No "Last used" sort exists — dropped, see spec.md.)
3. **Apply (US3)**: Click "Use" on that card — confirm the New Migration drawer opens with source/destination/mappings/options pre-filled and every field still editable. Submit the migration. **There is no usage counter to check afterward** (dropped, no backend field) — this step just confirms submission succeeds.
4. **Stale reference (US3 edge case, known gap)**: Delete the network/storage mapping referenced by a saved template, then click "Use" on it again — today the field is silently left unset/mismatched with **no inline warning** (FR-009 not implemented). If you're picking this up, this is the gap to close, not a regression to chase.
5. **Detail view (US4)**: Click a card (not "Use") — confirm the `DrawerShell`-based detail drawer opens, shows Created + full Source & Destination (incl. Tenant/project resolved live), Network & Storage Mappings + Copy method, and a Migration Options section (copy mode/cutover/guest OS/advanced), and closes cleanly via the X.
6. **Delete/Clone (US5)**: Hover a grid card (or look at a list row) — confirm clone/delete icons appear; clone a template, confirm the clone appears with a "(copy)" suffix and independent identity; delete the original — confirm any migration previously submitted from it is unaffected (still visible/runnable under the existing Migrations tab); delete the clone too and confirm it disappears from the Templates list.
7. **Regression — ephemeral lifecycle unaffected**: Open a *fresh* New Migration drawer (not via "Use"), fill in creds, and click Cancel — confirm (via `kubectl -n migration-system get migrationtemplates`) that the auto-created ephemeral (uuid-named) template is deleted as it always was. It's a different CRD (`MigrationTemplate`) from saved templates (`MigrationBlueprint`), so there's nothing to touch by construction — confirm `kubectl -n migration-system get migrationblueprints` still lists your saved template from step 1 untouched.
8. **Regression — retry flow unaffected**: Retry a previously failed/completed migration (not created from a saved template) and confirm the existing retry-prefill behavior is unchanged.
