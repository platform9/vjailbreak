---
title: Archived Releases
description: Archived Release Notes for vJailbreak
---

## v0.1.2
### What's Changed
* Upgrade min version to v0.1.1 by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/71
* Use version as tag for release event by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/72
* Added React Query to cache data, refactored MigrationForm component, and added error handling to MigrationForm by @knnguy in https://github.com/platform9/vjailbreak/pull/69
* Add FAQs by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/74
* Fix for incomplete transfer of changed blocks by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/75
* Add Video Demo by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/73
* Add vCenter role perms and port req by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/76
* Adding additional network port prereqs for NBDkit by @jeremymv2 in https://github.com/platform9/vjailbreak/pull/77
* replaced vCenter Role permissions with more comprehensive list from R… by @jeremymv2 in https://github.com/platform9/vjailbreak/pull/78
* update README with table for prereqs by @jeremymv2 in https://github.com/platform9/vjailbreak/pull/79
* Add debug logs to v2v-helper by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/80
* UI: Fixes by @knnguy in https://github.com/platform9/vjailbreak/pull/87
* Remove unused VMInfo field by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/82
* Add ephemeral storage req and limits by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/119
* Compress nmcli scripts by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/120
* Wait for volume mount for up to a minute by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/121
* Disable healthchecks by default by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/122
* Update healthcheck in README by @tanaypf9 in https://github.com/platform9/vjailbreak/pull/124
* Mapped OS_PROJECT_DOMAIN_NAME to OS_DOMAIN_NAME and OS_PROJECT_NAME t… by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/130
* add IpAddress and VM state info to migrationtemplate status by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/133
* 106 - disabled stopped VMs from migrating and notified with a tooltip by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/136
* Allowed one to many network mappings from VMware to Openstack by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/137
* UI: If the form is not submitted CRs created, should be deleted by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/139
* UI: Handle VM refresh with additional put call to migration template and add adjust the timeout needed for it to reflect latest detail by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/142
* check if disks have os installed in lexical order (release) by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/140

### Known Issues
* If the VM to be migrated has an LV spanning multiple physical devices used as boot volume unless both are mounted simultaneously to the OS-VM, vJailbreak cannot detect whether it is bootable, or not. ([issue link](https://github.com/platform9/vjailbreak/issues/146))
* If a user turns on a VM on VCenter after migration and tries to migrate it again, the migration object will not be created. In this case, the user should delete the VM from PCD/Openstack before trying the migration again. This error will be pushed up the stack for visibility.

### New Contributors
* @patil-pratik-87 made their first contribution in https://github.com/platform9/vjailbreak/pull/130
* @OmkarDeshpande7 made their first contribution in https://github.com/platform9/vjailbreak/pull/133

**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.1...16.01

##  v0.1.3
### What's Changed
* Fix ui to accept OS_INSECURE from rc file by @spai-p9  in https://github.com/platform9/vjailbreak/pull/177


**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.2...v0.1.3

## v0.1.4
### What's Changed
* Fix GH workflow runs on release event by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/147
* workflow dispatch manual by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/148
* Update README.md by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/149
* Update model.ts by @markp93 in https://github.com/platform9/vjailbreak/pull/152
* Added delete migration functionality to the migration table by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/155
* Adding workers nodes to the vjailbreak by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/161
* update package.json by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/172
* Update issue templates by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/158
* Update bug_report.yaml by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/175
* Migration data copy progress % and migration phases by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/167
* Nit: consistent referring to company VMware by @ericwb in https://github.com/platform9/vjailbreak/pull/164
* Update README.md by @markp93 in https://github.com/platform9/vjailbreak/pull/170
* release version v0.1.4 (release) by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/196

### New Contributors
* @markp93 made their first contribution in https://github.com/platform9/vjailbreak/pull/152
* @ericwb made their first contribution in https://github.com/platform9/vjailbreak/pull/164

**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.3...v0.1.4

## v0.1.5
### What's Changed
* Accept vmware and openstack creds via secert by @spai-p9 in https://github.com/platform9/vjailbreak/pull/228
* Create and Reuse existing open stack and vmware creds with other mino… by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/237
* Support OS installed on LV by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/227
* Adding templates for github wiki by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/226
* resolving lint issues by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/239
* resolving lint issues by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/240
* update image tag to v0.1.5 by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/242
* Base documentation update for vJailbreak by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/243
* Update checkout action to generate release notes by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/249
* Update checkout action to generate release notes by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/250
* Ingress changes to call k8s apis by @spai-p9 in https://github.com/platform9/vjailbreak/pull/252
* Reformatted readme by @damian-pf9 in https://github.com/platform9/vjailbreak/pull/206

### New Contributors
* @damian-pf9
* @anmolsachan
* @bhavin192

**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.4...v0.1.5

## v0.1.6
### What's Changed
* Update the correct gh-pages by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/265
* Update the correct icon for gh-pages by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/266
* Update the correct version for gh-pages by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/267
* Bug :: #269 :: Unable to use uppercase letters in a credential's name by @AbhijeetThakur in https://github.com/platform9/vjailbreak/pull/273
* fix insecure openstack authentication (release) by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/278
* Fixed issues with openstack cred creation + validation by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/280

### New Contributors
* @AbhijeetThakur made their first contribution in https://github.com/platform9/vjailbreak/pull/273

**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.5...v0.1.6

## v0.1.7
### What's Changed
* Update video link to latest by @anmolsachan in https://github.com/platform9/vjailbreak/pull/290
* fix readme for openstackcreds and vmware creds by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/288
* ( release ) Change base image to ubuntu base image, reduced image size.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/286
* Ignore Failed phase on retry by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/289
* api cosmetic fixes by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/274
* Support rollback for failed VM by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/300
* check VM status before marking migration as complete by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/295
* UI: Api cosmetic changes UI by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/297
* UI: Fix active migration by @spai-p9 in https://github.com/platform9/vjailbreak/pull/304
* fix mocks, unit tests by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/308
* block openstackcreds deletion for master creds by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/303
* UI: Implement a new credential addition workflow, and remove the creds addition from MigrationForm and Scaleup drawer by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/298
* Easy debug config by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/287
* resolved lint issues by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/311
* 270 identically named source destination credentials arent displayed properly by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/312
* Actions enhancement: Build and push on every PR raised to main or release branch.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/313
* Heavily re-organized docs pages by @damian-pf9 in https://github.com/platform9/vjailbreak/pull/309
* Update the docs github page by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/314
* Sync VDDK across agents without any manual intervention  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/296
* Fix versions and github actions by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/316
* Fixed regression while updating cosmetic api changes, virtualmac… by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/317
* 0.1.7 release fixes by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/318
* Build: Fix CGO Linking for libnbd Library in Build Process (release) by @spai-p9 in https://github.com/platform9/vjailbreak/pull/315
* fix migration status with random pod ref name by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/320
* UI: Disable scale down of a node when active migration is going on by @spai-p9 in https://github.com/platform9/vjailbreak/pull/319
* release: Fix github action when PR raised from release branch by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/323


**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.6...v0.1.7

## v0.1.8
### What's Changed
* Release v0.1.7 by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/322
* Update release notes by @sharma-tapas in https://github.com/platform9/vjailbreak/pull/326
* Updating the video by @roopakparikh in https://github.com/platform9/vjailbreak/pull/342
* Update the readme to point to the 'Getting Started' by @roopakparikh in https://github.com/platform9/vjailbreak/pull/349

### New Contributors
* @roopakparikh made their first contribution in https://github.com/platform9/vjailbreak/pull/342

**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.6...v0.1.8

## v0.1.9
### What's Changed
* Backend: Add check if datacenter exists by @spai-p9 in https://github.com/platform9/vjailbreak/pull/368
* Proxy env injection via configmap by @spai-p9 in https://github.com/platform9/vjailbreak/pull/364
* removed creds from logs and Made changes to match OS_INSECURE value t… by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/379
* vddk files check before migration ( release ) by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/375
* Damian doc updates by @damian-pf9 in https://github.com/platform9/vjailbreak/pull/377
* Doc: Inject any environment variables required by user into v2v-helper pod.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/381
* Backend: Change ownership of /home/ubuntu/vmware-vix-disklib-distrib to ubuntu:ubuntu via init container.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/378
* ensure correct permissions on shared vmwarelib directory for rsync ac… by @spai-p9 in https://github.com/platform9/vjailbreak/pull/383
* Scaling doc update by @damian-pf9 in https://github.com/platform9/vjailbreak/pull/391
* Backend: Remove logs in vmwaremachines  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/390

### New Contributors
* @sarika-p9 made their first contribution in https://github.com/platform9/vjailbreak/pull/379

**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.8...v0.1.9

## v0.1.10
### What's Changed
* added os icons in vms list table in Migrations by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/431
* Backend change to block selection of vms without appropriate flavors in openstack by @spai-p9 in https://github.com/platform9/vjailbreak/pull/424
* Backend:Retrieve correct network names from backing Network references instead of device summary by @spai-p9 in https://github.com/platform9/vjailbreak/pull/432
* Backend: Improve the error handling and event reporting on these failures.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/411
* Backend: Preserve the job logs.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/435


**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.9...v0.1.10
