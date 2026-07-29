# vjb-debug Skill

AI skill for debugging vJailbreak (VJB) VMware-to-PCD VM migration failures.

## When to Use

Invoke this skill (via `Skill tool: "vjb-debug"`) when:

- Migration is stuck or failed in any phase (discovery, mapping, validate, data copy, convert, cutover, post-migration)
- VMware or OpenStack credential failures / revalidation loops
- Network or storage mapping errors, subnet mismatch, port-already-in-use
- nbdcopy / NFC copy failures
- Storage-Accelerated Copy (XCOPY) failures on Pure Storage or NetApp
- Hot-Add Proxy failures
- virt-v2v conversion errors (resolv.conf immutable, dynamic disk/LDM, Hivex errors)
- Windows disks offline after migration, VMware Tools residual artifacts
- Admin cutover not progressing
- Windows Failover Cluster (WSFC) issues post-migration (NetFT adapter missing, cluster IP unreachable)
- Agent scaling problems
- Cluster Conversion (ESXi-to-PCD-hypervisor) issues

## Skill Entry Point

[SKILL.md](SKILL.md) — main debugging workflow: get migration name → classify phase → route to reference doc → investigate → decide retry/fix.

## Reference Docs

| File | What it covers |
|------|---------------|
| [architecture.md](architecture.md) | Pods, CRDs, credentials (VMwareCreds/OpenstackCreds), ConfigMap settings, agent scaling, compatibility matrix, known limitations |
| [migration-lifecycle.md](migration-lifecycle.md) | Phase-by-phase flow diagram, retry vs. cleanup decision tree, cutover, post-migration options |
| [copy-methods.md](copy-methods.md) | Normal NFC (nbdcopy), Storage-Accelerated XCOPY (Pure Storage / NetApp), Hot-Add Proxy — setup, failure modes, troubleshooting |
| [networking.md](networking.md) | Network mapping, IP/MAC/interface persistence across migration, WSFC/Neutron anti-spoofing case study |
| [guest-os-issues.md](guest-os-issues.md) | Windows and Linux conversion quirks: resolv.conf, dynamic disks, PCI slots, VMware Tools cleanup, NetFT adapter |
| [cluster-conversion.md](cluster-conversion.md) | ESXi-to-PCD-hypervisor (Cluster Conversion) — discovery, known regressions, maturity caveats |
| [support-bundle.md](support-bundle.md) | Log and CRD location map, `kubectl` commands for support bundles, support-bundle ZIP layout |

## Key Concepts

- **Correlation ID**: migration name → `Migration` CRD name → `<migration-name>-v2v-helper` pod → `/var/log/pf9/<migration-name>.log`
- **Three domains**: VMware side (vCenter/ESXi) · vJailbreak VM (k3s + controller + v2v-helper) · PCD/OpenStack (Nova/Neutron/Cinder)
- **Data copy transports**: Normal NFC · Storage-Accelerated XCOPY (cold-only, same-array) · Hot-Add Proxy

## External References

- vJailbreak docs: https://platform9.github.io/vjailbreak/
- Architecture deep-dive: https://deepwiki.com/platform9/vjailbreak
- virt-v2v: https://libguestfs.org/virt-v2v.1.html
- virt-v2v support matrix: https://libguestfs.org/virt-v2v-support.1.html
