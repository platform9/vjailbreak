---
title: Benchmarking
description: Standard Copy migration times measured on a vJailbreak appliance
---

Times for cold migrations using Standard Copy, with 100 GB of disk per VM.

| VM | Copy | Conversion | End to end |
|---|---|---|---|
| RHEL 8 | 13m08s | 3m46s | 20m04s |
| RHEL 9 | 12m42s | 3m59s | 19m46s |
| Rocky 9 | 11m44s | 4m08s | 18m57s |
| Debian 12 | 11m20s | 2m58s | 17m15s |
| Ubuntu 22.04 | 11m52s | 3m11s | 18m09s |
| Windows 2016 | 11m55s | 4m08s | 19m13s |
| Windows 2019 | 11m52s | 4m19s | 19m10s |
| Windows 2022 | 11m45s | 5m07s | 20m35s |

## vJailbreak Accelerated Copy


| VM | Copy | Conversion | End to end |
|---|---|---|---|
| RHEL 8 | 7m35s | 3m44s | 14m29s |
| RHEL 9 | 7m11s | 3m57s | 14m13s |
| Rocky 9 | 6m46s | 4m10s | 14m02s |
| Debian 12 | 6m25s | 3m00s | 12m21s |
| Ubuntu 22.04 | 6m51s | 3m09s | 13m06s |
| Windows 2016 | 6m44s | 4m10s | 14m05s |
| Windows 2019 | 6m51s | 4m22s | 14m12s |
| Windows 2022 | 6m39s | 5m04s | 15m26s |

End to end includes about 2 minutes of setup before the copy and about 45 seconds to create the VM.

## Measurement method

- Each VM was migrated once, one at a time, with the cold migration type and Standard Copy and later with vJailbreak accelerated Copy (`storageCopyMethod: normal`,`storageCopyMethod: HotAdd` ).
- The appliance node had 4 vCPU and 7 GiB of memory, with KVM available. The migration pod was limited to 2 CPU cores and 5 GiB.
- Times come from the timestamps in the migration pod log: copy is the time to copy the disks, conversion is the time from the start of disk conversion until it finishes, and end to end runs from pod start to VM creation.
