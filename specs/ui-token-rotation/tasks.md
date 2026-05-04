# Tasks: UI Service Account Token Security

**Input**: Design documents from `specs/ui-token-rotation/`  
**Branch**: `ui-token-rotation`  
**Base**: Cherry-pick or merge changes from `private/main/omkard/ui-hide-rotate-token` first (US1 already implemented there)

**Organization**: Tasks grouped by user story. US1 + k3s work already exists on `private/main/omkard/ui-hide-rotate-token` — T001 brings those changes onto this branch. Tasks marked `[via T001]` are fulfilled by completing T001; they do not need to be re-implemented. US2 and US3 tasks require new code on this branch.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (US1, US2, US3)
- **[via T001]**: Code already exists on `private/main/omkard/ui-hide-rotate-token`; fulfilled by completing T001 (merge) — no re-implementation needed

---

## Phase 1: Setup

**Purpose**: Bring the feature branch up to date with already-completed work.

- [x] T001 Merge or cherry-pick commits from `private/main/omkard/ui-hide-rotate-token` onto `ui-token-rotation` branch (2 commits: `141f3efb`, `e2947bf7`)

**Checkpoint**: All US1 + k3s changes are now on this branch. Tasks T002–T009 are fulfilled — no further action needed for them.

> **Note**: If the merge produces conflicts (both branches touch `axios.ts`, `pods.ts`), resolve in favour of the changes from `private/main/omkard/ui-hide-rotate-token` — they remove the token from the frontend entirely.

---

## Phase 2: Foundational (Blocking Prerequisite)

**Purpose**: The k3s configuration must be baked into the appliance image before projected tokens are useful — without `service-account-extend-token-expiration=false`, k3s silently extends token lifetimes and defeats the TTL.

- [ ] T002 [via T001] Add `image_builder/configs/k3s.config.yaml` with `service-account-extend-token-expiration=false`, `secrets-encryption: true`, and explicit issuer/signing-key flags
- [ ] T003 [via T001] Update `image_builder/vjailbreak-image.pkr.hcl` to provision `k3s.config.yaml` to `/etc/rancher/k3s/config.yaml` with correct ownership/permissions

**Checkpoint**: k3s will now respect `expirationSeconds` without silent extension. Foundation for US2 is ready.

---

## Phase 3: User Story 1 — Token Never Reaches the Browser (Priority: P1) 🎯 MVP

**Goal**: All Kubernetes API calls are proxied server-side by Nginx with the SA token injected via Lua. The browser never sees or sends a ServiceAccount token.

**Independent Test**: Load the UI in a browser. Open DevTools → Network tab and page source. Search for the string `eyJ` (JWT prefix) — it must not appear anywhere in requests, responses, or HTML. All UI operations (list migrations, view pods, stream logs) must still work.

### Implementation for User Story 1

- [ ] T004 [via T001] [P] [US1] Update `ui/default.conf` — add Lua `set_by_lua_block` that reads token from `/var/run/secrets/kubernetes.io/serviceaccount/token` on each request; add `proxy_set_header Authorization "Bearer $sa_token"` and TLS verification via `proxy_ssl_trusted_certificate`; add proxy locations for `^~ /api/`, `^~ /apis/`, `^~ /sdk/` with `proxy_buffering off`; add WebSocket upgrade headers (`Upgrade`, `Connection`)
- [ ] T005 [via T001] [P] [US1] Simplify `ui/startup.sh` — remove the `envsubst` token injection block; file should only start OpenResty
- [ ] T006 [via T001] [P] [US1] Update `ui/src/api/axios.ts` — remove `Authorization: Bearer ${authToken}` from `getHeaders()`; frontend sends no auth header
- [ ] T007 [via T001] [P] [US1] Update `ui/src/api/kubernetes/pods.ts` — remove `Authorization` header from the raw `fetch()` call in `streamPodLogs` (lines 58–61)
- [ ] T008 [via T001] [P] [US1] Update `ui/vite.config.ts` — make `VITE_API_TOKEN` optional in dev proxy headers (only add `Authorization` header if env var is set)
- [ ] T009 [via T001] [P] [US1] Update `ui/README.md` — mark `VITE_API_TOKEN` as optional; note it is only used in dev mode

**Checkpoint**: User Story 1 complete. Build the UI container and verify no token appears in browser traffic. All API calls succeed through the Nginx proxy.

---

## Phase 4: User Story 2 — Token Has a Bounded Lifetime (Priority: P2)

**Goal**: The SA token file on disk (read by Nginx) is issued with a configurable TTL and automatically rotated by kubelet before expiry. A leaked server-side token has a bounded useful lifetime.

**Independent Test**: Deploy the updated manifest. Check the token file: `kubectl exec -n migration-system <ui-pod> -- cat /var/run/secrets/kubernetes.io/serviceaccount/token | cut -d. -f2 | base64 -d | python3 -m json.tool | grep exp` — the `exp` field must be within `expirationSeconds` of the current time, not years in the future.

### Implementation for User Story 2

- [x] T010 [US2] Update `deploy/07ui-deployment.yaml` — add `automountServiceAccountToken: false` to the pod spec; add projected volume `sa-token` with three sources: `serviceAccountToken` (`expirationSeconds: 86400`, `path: token`), `configMap` (`kube-root-ca.crt` → `ca.crt`), `downwardAPI` (`metadata.namespace` → `namespace`); add volumeMount for `sa-token` at `/var/run/secrets/kubernetes.io/serviceaccount` (`readOnly: true`)
- [x] T011 [US2] Run `make generate-manifests` from repo root to regenerate `deploy/installer.yaml`; verify the Deployment section in `installer.yaml` shows `automountServiceAccountToken: false` and the `sa-token` projected volume; if `make generate-manifests` does not pick up the Deployment change, manually update the corresponding Deployment block in `installer.yaml` to match `07ui-deployment.yaml`

**Checkpoint**: User Story 2 complete. The token on disk now has `exp` set to ~24h from issuance. Nginx reads it per-request — when kubelet rotates the file (~80% of TTL), the next request automatically uses the fresh token with no restart.

---

## Phase 5: User Story 3 — Configurable Token TTL (Priority: P3)

**Goal**: An administrator can change how long the token remains valid by editing a single field in the Deployment and rolling the pod. The process is documented.

**Independent Test**: Change `expirationSeconds` to `3600` (1 hour), roll the Deployment, decode the new token's `exp` — it must be ~1 hour from now, not 24 hours.

### Implementation for User Story 3

- [x] T012 [US3] Add an inline comment above `expirationSeconds: 86400` in `deploy/07ui-deployment.yaml` noting the minimum value (600s), the default (86400s), and that a Deployment rollout is required to apply changes
- [x] T013 [P] [US3] Update `docs/superpowers/specs/2026-05-04-ui-token-rotation-design.md` — correct the stated minimum TTL from 3600s to 600s (research finding); add a section on TTL change procedure for operators

**Checkpoint**: User Story 3 complete. Operator has clear instructions for adjusting TTL.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Verify the full system works end-to-end and no regressions were introduced.

- [ ] T014 [P] Functional smoke test ⚠️ MANUAL — requires live cluster — deploy the updated image + manifests; verify all UI pages load and all k8s API calls succeed: list migrations, list VMs, view pod logs (streaming), check credentials; confirm no `401 Unauthorized` errors
- [ ] T015 [P] Browser security check ⚠️ MANUAL — requires live cluster + browser — open DevTools → Network; trigger each API call type; confirm zero requests contain `Authorization: Bearer` header from the browser side; confirm page source contains no JWT string (`eyJ` prefix)
- [ ] T016 [P] WebSocket / log streaming check ⚠️ MANUAL — requires live cluster + browser — open a migration log stream in the UI; confirm it streams without error through the Nginx WebSocket proxy
- [x] T017 Update `specs/ui-token-rotation/research.md` — mark all decisions as implemented; note any deviations from the plan

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — k3s flag must be baked in before projected tokens are effective
- **Phase 3 (US1)**: Depends on Phase 1 only — Nginx proxy work is independent of k3s config
- **Phase 4 (US2)**: Depends on Phase 3 (proxy must be in place before disabling automount; otherwise token is gone with no replacement)
- **Phase 5 (US3)**: Depends on Phase 4 — TTL config only meaningful once projected volume exists
- **Phase 6 (Polish)**: Depends on Phases 3–5

### User Story Dependencies

- **US1 (P1)**: Independent — can complete without US2/US3
- **US2 (P2)**: Depends on US1 — do not disable automount until proxy is verified working (would break the UI)
- **US3 (P3)**: Depends on US2 — TTL config only meaningful once projected volume is in place

### Parallel Opportunities Within Phases

- T002, T003 (Phase 2): Can run in parallel — different files
- T004–T009 (Phase 3 / US1): All can run in parallel — different files
- T012, T013 (Phase 5 / US3): Can run in parallel — different files
- T014, T015, T016 (Phase 6): All can run in parallel — independent verification tasks

---

## Parallel Example: User Story 1 (Phase 3)

```text
# All US1 tasks touch different files — run in parallel:
T004: ui/default.conf        (Nginx proxy + Lua)
T005: ui/startup.sh          (remove token injection)
T006: ui/src/api/axios.ts    (remove Authorization header)
T007: ui/src/api/kubernetes/pods.ts  (remove Authorization header)
T008: ui/vite.config.ts      (VITE_API_TOKEN optional)
T009: ui/README.md           (doc update)
```

---

## Implementation Strategy

### MVP (Minimum Viable Security Fix)

1. Complete Phase 1: Merge user's branch
2. Complete Phase 3 (US1): Token never reaches browser — **this is the highest-impact change**
3. **STOP and VALIDATE**: Browser inspection test passes, all API calls work
4. This alone is a significant security improvement — deploy if needed

### Full Implementation

1. Complete Phase 1 + 2 (Setup + k3s config)
2. Complete Phase 3 (US1) → validate
3. Complete Phase 4 (US2) → validate token TTL
4. Complete Phase 5 (US3) → validate TTL configurability
5. Complete Phase 6 (Polish) → full regression check

### Key Risk: Order of US1 and US2

**Do not apply Phase 4 (US2) before Phase 3 (US1) is deployed and verified.** Disabling `automountServiceAccountToken` without the Nginx proxy in place will cause the UI to lose all k8s API access immediately. The proxy must be live and confirmed working before the automount is disabled.

---

## Notes

- Tasks marked `[via T001]` exist on `private/main/omkard/ui-hide-rotate-token` — completing T001 (the merge) brings them onto this branch; no re-implementation needed
- The only new code to write is T010 (projected volume in Deployment YAML) and T011 (installer.yaml sync)
- All other remaining tasks (T012–T017) are documentation, comments, and verification
- Total implementation effort is minimal — the hard work is already done
