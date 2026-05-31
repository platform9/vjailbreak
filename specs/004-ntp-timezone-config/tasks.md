# Tasks: NTP Server and Timezone Configuration

**Input**: Design documents from `specs/004-ntp-timezone-config/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓

## Format: `[ID] [Status] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Status]**: ✅ Done | 🔲 Pending | ⚠️ Needs verification
- **[Story]**: US1=Timezone | US2=NTP | US3=Combined | US4=View

---

## Phase 1: Shared Package (`pkg/common/timesettings/`)

**Goal**: Pure logic for NTP validation, TZ sanitization, conf-file writes, D-Bus calls, and Kubernetes effects — all in one testable package.

- [x] T001 ✅ [US1,US2] Create `pkg/common/timesettings/timesettings.go` with: `IsValidNTPServer`, `FilterValidNTPServers`, `sanitizeTimezone`, `writeTimesyncdConf`, `updatePf9EnvFile`, `notifyTimedateViaDbus`, `restartTimesyncdViaDbus`, `patchPf9EnvConfigMap`, `restartTZWorkloads`, `patchVersionCheckerTZ`, `Apply()`
- [x] T002 ✅ [US1,US2] Write table-driven unit tests in `pkg/common/timesettings/timesettings_test.go` covering: `TestIsValidNTPServer` (IPv4, hostname, empty, URL-form, path-form, label>63 chars), `TestFilterValidNTPServers` (comma/newline/space, mixed valid/invalid), `TestSanitizeTimezone_*` (traversal, valid, empty, leading slash), `TestWriteTimesyncdConf` (creates dir+file, removes on empty), `TestUpdatePf9EnvFile` (creates, replaces TZ=, appends, empty→UTC)
- [x] T003 ✅ Add `migration-vpwned-sdk` to `workloadsToRestart` in `timesettings.go` so it restarts AFTER `patchPf9EnvConfigMap` — fixes TZ=UTC race condition
- [x] T004 ✅ Remove deferred goroutine from `ApplyTimeSettings` handler in `vjailbreak_proxy.go` — was causing self-kill race
- [x] T005 ✅ Remove `RestartDeployment` exported function from `timesettings.go` (dead code after goroutine removal)

**Checkpoint**: `cd pkg/common && go test ./timesettings/... -v` all pass. ✅

---

## Phase 2: gRPC API (`pkg/vpwned/`)

**Goal**: Expose `ApplyTimeSettings` RPC via gRPC + HTTP gateway.

- [x] T006 ✅ Add `ApplyTimeSettings` RPC, `ApplyTimeSettingsRequest`, `ApplyTimeSettingsResponse` to `pkg/vpwned/sdk/proto/v1/api.proto`
- [x] T007 ✅ Regenerate gRPC bindings: `api.pb.go`, `api.pb.gw.go`, `api_grpc.pb.go` in `pkg/vpwned/api/proto/v1/service/`
- [x] T008 ✅ Implement `ApplyTimeSettings` handler in `pkg/vpwned/server/vjailbreak_proxy.go`: calls `timesettings.Apply()`, returns response on success, error on hard failure only

**Checkpoint**: `cd pkg/vpwned && go build ./...` succeeds. ✅

---

## Phase 3: AppArmor Annotation

**Goal**: Allow D-Bus calls from the vpwned-sdk container to reach the host system bus.

- [x] T009 ✅ Add `container.apparmor.security.beta.kubernetes.io/vpwned: unconfined` to pod template annotations in `deploy/06vpwned-deployment.yaml`
- [x] T010 ✅ Add same AppArmor annotation to pod template in `k8s/migration/config/addons/k8s.svc.yaml`
- [x] T011 ✅ Add same AppArmor annotation to pod template in `pkg/vpwned/deploy/k8s.svc.yaml`

**Checkpoint**: `kubectl get pod -n migration-system -l app=vpwned-sdk -o jsonpath='{.items[0].metadata.annotations}'` shows `container.apparmor.security.beta.kubernetes.io/vpwned: unconfined` after deployment.

---

## Phase 4: UI

**Goal**: Administrators can view and change timezone and NTP servers in the Global Settings UI.

- [x] T012 ✅ [US4] Add `TIMEZONE?: string` and `NTP_SERVERS?: string` to `data` in `VjailbreakSettings` interface in `ui/src/api/settings/model.ts`
- [x] T013 ✅ [US4] Add `applyTimeSettings(): Promise<void>` to `ui/src/api/settings/settings.ts` — POST `/dev-api/sdk/vpw/v1/settings/apply-time-settings`
- [x] T014 ✅ [US1,US2] Add `TIMEZONE: string` and `NTP_SERVERS: string` to `SettingsForm` in `ui/src/features/globalSettings/helpers.ts`; update `toConfigMapData` and `fromConfigMapData` to include both fields
- [x] T015 ✅ [US1] Create `ui/src/features/globalSettings/timezones.ts` — static array of IANA timezone identifiers
- [x] T016 ✅ [US2] Create `ui/src/features/globalSettings/validators.ts` — UI-side NTP server validation (mirrors backend rules)
- [x] T017 ✅ [US1,US2] Update `ui/src/features/globalSettings/components/GlobalSettingsPage.tsx`: add timezone autocomplete field + NTP servers text input; call `applyTimeSettings()` only when `timeSettingsChanged`; always show `"Time settings applied successfully."` on success
- [x] T018 ✅ Remove `hasWarnings` conditional branch from `GlobalSettingsPage.tsx` — warnings go to server logs only, never shown in UI

**Checkpoint**: `cd ui && yarn build` succeeds. ✅

---

## Phase 5: End-to-End Verification

**Goal**: Confirm all changes work correctly on a live appliance VM.

- [ ] T019 🔲 Build and deploy new vpwned image with all fixes to test VM
- [ ] T020 ⚠️ [US1] Apply timezone `Asia/Calcutta` → verify `timedatectl` shows IST on host
- [ ] T021 ⚠️ [US1] After apply → verify `kubectl -n migration-system get cm pf9-env -o jsonpath='{.data.TZ}'` = `Asia/Calcutta`
- [ ] T022 ⚠️ [US1] After workloads restart → verify `kubectl -n migration-system exec deploy/migration-vpwned-sdk -- cat /proc/1/environ | tr '\0' '\n' | grep TZ` = `TZ=Asia/Calcutta`
- [ ] T023 ⚠️ [US1] After workloads restart → verify controller-manager shows IST timestamps in logs (not UTC)
- [ ] T024 ⚠️ [US2] Apply NTP server `pool.ntp.org` → verify `/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf` contains `NTP=pool.ntp.org`
- [ ] T025 ⚠️ [US2] After NTP apply → verify `timedatectl show | grep NTPSynchronized` = `NTPSynchronized=yes`
- [ ] T026 ⚠️ [US3] Apply both timezone + NTP → verify all of T020–T025 simultaneously
- [ ] T027 ⚠️ [US3] Clear both fields → verify UTC restored, conf file deleted, `timedatectl show` shows NTP=no or default
- [ ] T028 ⚠️ [US4] Reload Global Settings page after apply → verify configured timezone and NTP servers appear in form fields
- [ ] T029 🔲 [US2] Enter invalid NTP entries (`http://a.com,valid.ntp.org`) → apply → verify only `valid.ntp.org` in conf file; no UI error shown
- [ ] T030 🔲 [US1] Enter timezone traversal string `../../etc/passwd` in UI → verify save is blocked by frontend validation or sanitized by backend

**Checkpoint**: All T020–T030 pass. Feature verified end-to-end. ✅

---

## Dependencies & Execution Order

- **Phase 1 → Phase 2**: T006–T008 depend on `timesettings.Apply()` signature from T001
- **Phase 3**: Independent — can run in parallel with Phase 1 and 2
- **Phase 4**: Independent — can run in parallel with Phases 1–3
- **Phase 5**: Depends on all prior phases deployed to test VM

### Parallel Opportunities

- T009, T010, T011 (Phase 3): Different files — parallel
- T012–T016 (Phase 4): Different files — parallel within UI
- T020–T029 (Phase 5): Run sequentially for clear state isolation

---

## Task Count Summary

| Phase | Tasks | Status |
|-------|-------|--------|
| Phase 1: Shared Package | 5 (T001–T005) | ✅ Complete |
| Phase 2: gRPC API | 3 (T006–T008) | ✅ Complete |
| Phase 3: AppArmor | 3 (T009–T011) | ✅ Complete |
| Phase 4: UI | 7 (T012–T018) | ✅ Complete |
| Phase 5: Verification | 12 (T019–T030) | 🔲 Pending |
| **Total** | **30** | |

---

## Remaining Blockers

1. **T019**: New vpwned image must be built and deployed with these fixes:
   - `migration-vpwned-sdk` in `workloadsToRestart`
   - Deferred goroutine removed from `ApplyTimeSettings`
   - AppArmor annotation on pod template

2. **T022 / T023**: Without the new image, vpwned-sdk will still show `TZ=UTC` after apply — this is the primary symptom reported by the user.

3. **Verification commands**:
```bash
# Use /proc/1/environ instead of `env` — works in distroless containers
kubectl -n migration-system exec deploy/migration-vpwned-sdk -- \
  cat /proc/1/environ | tr '\0' '\n' | grep TZ

kubectl -n migration-system exec deploy/migration-controller-manager -- \
  cat /proc/1/environ | tr '\0' '\n' | grep TZ
```
