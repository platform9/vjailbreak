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
## v0.1.11

### What's Changed
* added swagger ui for v0.1.9 and v0.1.10 by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/439
* counter output for incremental copy by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/451
* automated swagger ui at every push by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/450
* Migration stuck in Pending on UI by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/459
* Wait for volume attachements also to clear out. by @spai-p9 in https://github.com/platform9/vjailbreak/pull/461
* v2v-helper: Add check if subnet exists to avoid panic.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/460
* enable openstack re-auth by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/464
* use flavor id instead of name in label by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/465


**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.10...v0.1.11
## v0.1.12

### What's Changed
* save migration debug logs at hostPath '/var/log/pf9' ( release ) by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/479
* Add ons in v2v-helper for post-migration actions by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/490
* Updated documentation for all external connectivity required for vjailbreak by @sarika-p9 in https://github.com/platform9/vjailbreak/pull/467
* Sync migration logs from worker to master ( release ) by @spai-p9 in https://github.com/platform9/vjailbreak/pull/494
* User should be able to delete migration when in pending or any stage by @spai-p9 in https://github.com/platform9/vjailbreak/pull/511
* added the admincutover option to the migration strategy by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/514
* disabled migration form submission on enter key when text inputs are … by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/515
* added insecure option to the vmware creds form by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/516
* Add check from nova side also for volume dettach  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/480
* Document: Add cbt privilege and flavor selection logic in Doc.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/502
* Increase timeout for vm status to become active by @spai-p9 in https://github.com/platform9/vjailbreak/pull/525
* create delete update vmwaremachines by @spai-p9 in https://github.com/platform9/vjailbreak/pull/523
* fix build issue.  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/526


**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.11...v0.1.12
## v0.1.13

### What’s Changed
* vPwned: Rolling Conversion (release) by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/546
* uncomment build-vpwned action by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/552
* added the functionality to validate and assign/edit ips to/of the vms for rolling conversion by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/567
* added the target cluster selections for migrations by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/573
* convert PCD Cluster name to k8s compatible by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/576
* fix issues when VM is attached to same network twice by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/579
* Release v0.1.13 (release) by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/581
* Add proxy for vjailbreak  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/562
* add/update/edit  os and flavors in vms list in rolling conversion by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/586
* Add alpine docker image with rsync,coreutils in quay  by @spai-p9 in https://github.com/platform9/vjailbreak/pull/590
* DOC: Add doc for virtio drivers injection by the user by @spai-p9 in https://github.com/platform9/vjailbreak/pull/589
* change from cloud-ctl to pcdctl by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/599
* Change the base ubuntu image for vJailbreak qcow by @spai-p9 in https://github.com/platform9/vjailbreak/pull/598
* Fix scale up by @spai-p9 in https://github.com/platform9/vjailbreak/pull/601
* Fix cluster name by @spai-p9 in https://github.com/platform9/vjailbreak/pull/602
* fix openstack volume sizes being smaller than original disk size by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/558
* improved guestfish logging by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/565
* Rolling Conversion UI changes by @patil-pratik-87 in https://github.com/platform9/vjailbreak/pull/548
* fix debug log file for migration re-runs by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/560
* move os family selection to per VM basis by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/568
* save disk space on vjb VM by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/571
* Prebake components by @spai-p9 in https://github.com/platform9/vjailbreak/pull/574
* Give users option to upload virtio drivers to a path in master and then propogate it down to agents by @spai-p9 in https://github.com/platform9/vjailbreak/pull/575
* make delete migration-controller-manager pod work by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/572

#### Usability Enhancements and Stability Improvements

- **Rolling Conversion (Beta Availability)**  
  The Rolling Conversion feature is now available as a beta release.

- **Pre-Packaged vJailbreak Image**  
  A new vJailbreak image is now available, containing all required components pre-installed. This enhancement eliminates the need for internet access during installation, making it suitable for restricted network environments.

- **Support for Custom VirtIO Drivers**  
  Users may now upload VirtIO drivers to a designated path. If present, these drivers will be utilized during the migration process. If absent, the system will default to downloading the necessary drivers from the internet, providing flexibility based on deployment conditions. https://github.com/platform9/vjailbreak/blob/main/docs/src/content/docs/guides/virtio_doc.md

- **Resolution of Pod Eviction Due to Disk Pressure**  
  An issue causing pod eviction under disk pressure conditions has been resolved. This fix improves the reliability and stability of workloads during extended migration operations.

- **Enhanced Debug Logging**  
  Debug logs have been improved to provide more comprehensive and structured output



**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.12...v0.1.13
## v0.1.14

### What's Changed
* revert the changes for duplicate networks by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/605
* accept os_family correctly and override if present by @spai-p9 in https://github.com/platform9/vjailbreak/pull/607
* support migration without a cluster on pcd by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/608
* do not fail migrations for snapshot delettions by @OmkarDeshpande7 in https://github.com/platform9/vjailbreak/pull/611


**Full Changelog**: https://github.com/platform9/vjailbreak/compare/v0.1.13...v0.1.14
