---
title: One-Stop Migration Networking
description: Find your migration networking situation, apply the right settings, and know exactly what the OpenStack port and the guest OS will look like afterward.
---

Network settings are the most common cause of failed or surprising migrations. This guide is organized by situation: find the scenario that matches yours, apply the settings, and check the expected outcome before you migrate.

Every scenario reports two separate outcomes:

- **The OpenStack port** — the address and MAC that OpenStack assigns to the network port.
- **Inside the guest** — what the migrated VM's own network configuration looks like on first boot.

These two can differ. A preserved IP address on the port does not always mean the guest is statically configured to match it.

:::note
For the underlying mechanism behind interface-name persistence and per-OS guest behavior, see [Network Persistence](../../../concepts/network-persistence/).
:::

## Advanced options for migration

| Setting | What it controls |
| --- | --- |
| **Preserve IP** | Carry the discovered source IP onto the OpenStack port. |
| **IP address box** | Leave as-is (discovered IP), type one IPv4 address, or leave empty. |
| **Preserve MAC** | Carry the source MAC, or let OpenStack generate a new one. |
| **Fallback to DHCP** | On = accept an OpenStack-assigned address rather than fail. Off = stop the migration on conflict or mismatch. |
| **Persist source network interfaces** | Restore the original interface names inside the guest. Only available when Preserve IP is on. |

## Table of scenarios

Find the situation that matches yours, read across the row for the settings to apply, then follow the link to the detailed block — expected outcome, port and guest behavior, and the caveats.

| Scenario | Preserve IP | IP address box | Preserve MAC | Fallback to DHCP | Persist source network interfaces | Details |
| --- | --- | --- | --- | --- | --- | --- |
| **A** — Same subnet: keep the exact IP and MAC, fail if the address is taken | **On** | Leave as-is | **On** | Off | **On** | [Scenario A →](#a--keep-the-same-ip-and-the-same-mac) |
| **B** — Specific new IP | Off | Type one IPv4 address | Either | Off, or on if a DHCP address is acceptable | Unavailable | [Scenario B →](#b--assign-a-specific-new-ip-address) |
| **C** — Everything on DHCP | Off on every NIC | Empty | Either | **On** — required | Unavailable | [Scenario C →](#c--force-everything-onto-dhcp) |
| **D** — Different subnet | Off | Empty, or an address valid in the new subnet | Either | **On** — required | Unavailable | [Scenario D →](#d--the-destination-is-on-a-different-subnet) |
| **E** — Same subnet, bulk move: prefer the IP but accept DHCP rather than fail | **On** | Leave as-is | **On** | **On** | **On** | [Scenario E →](#e--prefer-the-same-ip-but-accept-dhcp-rather-than-fail) |
| **F** — L2-only network | No effect on the port | Ignored | Either — on to keep the MAC | Greyed out | Your choice | [Scenario F →](#f--the-destination-is-an-l2-only-network) |
| **G** — Same IP, new MAC | **On** | Leave as-is | Off | Your choice | Your choice | [Scenario G →](#g--keep-the-ip-but-let-openstack-pick-a-new-mac) |
| **H** — Source VM powered off | Forced off (greyed out) | Type an address, or empty for DHCP | Either — usually on | On if the address box is empty | Unavailable | [Scenario H →](#h--the-source-vm-is-powered-off) |
| **I** — Port with no address | Off | Empty | Either | Off | Unavailable | [Scenario I →](#i--create-the-port-with-no-address-at-all) |
| **J** — Preserve IP and MAC, Persist Network off | **On** | Leave as-is | **On** | Your choice | Off | [Scenario J →](#j--preserve-ip-and-mac-but-persist-network-is-off) |

**Reading the table**

- **Either** / **Your choice** — the setting does not change the outcome the scenario describes.
- **Unavailable** — the UI disables **Persist source network interfaces** whenever Preserve IP is off.
- **Greyed out** — the UI disables the setting for this scenario; you cannot change it.
- **Leave as-is** — do not edit the box; it already shows the discovered IP.

:::caution
Scenarios **C** and **I** share every setting except **Fallback to DHCP**, and Scenarios **A**, **E**, and **J** share every setting except **Fallback to DHCP** and **Persist source network interfaces**. Those single differences change the result substantially, so confirm them against the detailed block before you migrate.
:::

---

## A — Keep the same IP and the same MAC

**Use this when**

- The destination network carries the same subnet as the source.
- The VM is powered on and its IP was discovered correctly.
- DNS records, firewall rules, or MAC-locked licenses depend on the address surviving the move.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | On |
| IP address box | Leave as-is (shows the discovered IP) |
| Preserve MAC | On |
| Fallback to DHCP | Off — you want the migration to stop rather than silently change the address |
| Persist source network interfaces | On — keeps the original interface names too |

**What you get**

- **OpenStack port:** the same IP address, provided nothing else in the network already holds it.
- **Port MAC:** the same MAC address.
- **Inside the guest:** unchanged. A static NIC stays static on the same address; a DHCP NIC stays on DHCP. The original interface names are restored.

:::caution
If the address is already in use, the migration fails with a port conflict. That is deliberate — it stops you from creating a duplicate. Consider ticking **Disconnect source network** so the original VM releases the address first.

**Persist source network interfaces** must be on for the "unchanged" guarantee above. If it is off, the guest outcome depends on the OS — see [Scenario J](#j--preserve-ip-and-mac-but-persist-network-is-off).
:::

## B — Assign a specific new IP address

**Use this when**

- You are re-addressing the VM as part of the move and already know the address it should get.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | Off |
| IP address box | Type the new address — one IPv4 address only |
| Preserve MAC | Either — your choice |
| Fallback to DHCP | Off if a wrong address should stop the migration; on if a DHCP address is an acceptable substitute |
| Persist source network interfaces | Unavailable |

**What you get**

- **OpenStack port:** the address you typed, provided it belongs to a subnet on the destination network.
  - If it does not belong to a subnet: the migration fails (Fallback off), or the port gets an OpenStack-assigned address (Fallback on).
- **Inside the guest:** DHCP configuration on the assigned address.

:::caution
**One address per NIC.** Two addresses are rejected with "Multiple IPs are not supported when Preserve IP is disabled".

A typed address carries no prefix length, so the guest assumes `/24`. Verify the netmask if your subnet is not a `/24`.
:::

## C — Force everything onto DHCP

**Use this when**

- You want a clean start: OpenStack allocates all addresses and no source addressing is carried over.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | Off on every NIC |
| IP address box | Empty |
| Preserve MAC | Either — your choice |
| Fallback to DHCP | On — required, see below |
| Persist source network interfaces | Unavailable |

**What you get**

- **OpenStack port:** an address allocated by OpenStack.
- **Port MAC:** preserved or newly generated, per your choice.
- **Inside the guest:** DHCP.

:::caution
**Fallback to DHCP must be on.** With it off, this same combination produces a port with no address at all — see [Scenario I](#i--create-the-port-with-no-address-at-all).
:::

## D — The destination is on a different subnet

**Use this when**

- The destination OpenStack network does not carry the source VM's subnet.

This is the most common cause of failed migrations.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | Off |
| IP address box | Empty (or type an address valid in the new subnet — see [Scenario B](#b--assign-a-specific-new-ip-address)) |
| Preserve MAC | Either — your choice |
| Fallback to DHCP | On — required |
| Persist source network interfaces | Unavailable — the UI greys it out automatically |

**What you get**

- **OpenStack port:** an address allocated by OpenStack from the destination subnet.
- **Port MAC:** the same MAC if Preserve MAC is on, otherwise a newly generated one.
- **Inside the guest:** DHCP.

:::caution
**Preserve IP on** with **Fallback to DHCP off** is the failure case — the port cannot be created and the migration stops, but only if the source NIC actually had an address. A NIC with no discovered IP is fine.

**Preserve IP on** with **Fallback to DHCP on** is tolerable: you still land on a DHCP address and keep the MAC without editing each NIC. Turning Preserve IP off is clearer.
:::

## E — Prefer the same IP, but accept DHCP rather than fail

**Use this when**

- Same subnet as Scenario A, but you are migrating in bulk and would rather a few VMs come up on a different address than have the whole batch stop.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | On |
| IP address box | Leave as-is |
| Preserve MAC | On |
| Fallback to DHCP | On |
| Persist source network interfaces | On |

**What you get**

- **OpenStack port:** the same IP where possible; an OpenStack-assigned address where not.
- **Port MAC:** the same MAC address.
- **Inside the guest:** the original static configuration where the address was preserved; DHCP where it was not.

:::caution
You will not be stopped when an address changes. Check the migration report afterward to see which VMs changed address.

As with Scenario A, the guest outcome above assumes **Persist source network interfaces** is on. If it is off, see [Scenario J](#j--preserve-ip-and-mac-but-persist-network-is-off).
:::

## F — The destination is an L2-only network

**Use this when**

- The destination OpenStack network is tagged as an L2 network and has no subnets.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | No effect on the port — no fixed IPs can be assigned |
| IP address box | Ignored |
| Preserve MAC | Works normally — keep it on if you need the MAC |
| Fallback to DHCP | Greyed out |
| Persist source network interfaces | Your choice |

**What you get**

- **OpenStack port:** created with no fixed IP addresses.
- **Port MAC:** preserved or newly generated, per your choice.
- **Inside the guest (Ubuntu with netplan):** a wildcard configuration puts every interface on DHCP.

:::caution
Nothing in vJailbreak assigns addresses on an L2 network. Make sure a DHCP server is reachable on that segment, or plan to configure the guest by hand.
:::

## G — Keep the IP but let OpenStack pick a new MAC

**Use this when**

- The source MAC clashes with something in the destination, or you are deliberately re-issuing hardware addresses.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | On |
| IP address box | Leave as-is |
| Preserve MAC | Off |
| Fallback to DHCP | Your choice — as in Scenarios A and E |
| Persist source network interfaces | Your choice |

**What you get**

- **OpenStack port:** the same IP address.
- **Port MAC:** a newly generated address. The UI shows a warning triangle next to the NIC to confirm this.
- **Inside the guest:** DHCP. The guest cannot match its old static settings to a hardware address it has never seen.

:::caution
Anything license-locked to the MAC address will break.

Interface names inside the guest may change, because names are recovered by matching on the MAC.
:::

## H — The source VM is powered off

**Use this when**

- The VM is not running, so VMware Tools reported no addresses.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | Forced off and greyed out — nothing to preserve |
| IP address box | Type the address you want, or leave empty for DHCP |
| Preserve MAC | Available, and usually worth keeping on |
| Fallback to DHCP | On if you left the address box empty |
| Persist source network interfaces | Unavailable |

**What you get**

- **OpenStack port:** your typed address, an OpenStack-assigned one, or no address if the box was empty with Fallback off.
- **Port MAC:** preserved. The MAC is read from the virtual NIC rather than the guest, so it survives a powered-off migration.
- **Inside the guest:** no IP configuration is injected if no address was requested.

:::tip
If you know the VM's address, power it on briefly before migrating so vJailbreak can discover it — that unlocks [Scenario A](#a--keep-the-same-ip-and-the-same-mac).
:::

## I — Create the port with no address at all

**Use this when**

- You intend to configure addressing yourself after the migration, and want the port attached but unaddressed.

Rarely used. If you reached this by accident, you probably wanted [Scenario C](#c--force-everything-onto-dhcp).

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | Off |
| IP address box | Empty |
| Preserve MAC | Either — your choice |
| Fallback to DHCP | Off |
| Persist source network interfaces | Unavailable |

**What you get**

- **OpenStack port:** created and attached, with no fixed IP.
- **Port MAC:** preserved or newly generated, per your choice.
- **Inside the guest:** that interface is skipped entirely. No address, no DHCP client, nothing.

:::caution
The VM will boot with an unconfigured NIC and no network reachability on it. Have console access ready.
:::

## J — Preserve IP and MAC, but Persist Network is off

**Use this when**

- You want the address and MAC carried over, but do not need the original interface names restored.
- Or **Persist source network interfaces** was simply left at its default (off) and you want to know what to expect.

This is the case Scenarios A and E refer to. The port outcome is identical to them — only the guest differs.

**Settings**

| Setting | Value |
| --- | --- |
| Preserve IP | On |
| IP address box | Leave as-is |
| Preserve MAC | On |
| Fallback to DHCP | Your choice — as in Scenarios A and E |
| Persist source network interfaces | Off |

**What you get**

- **OpenStack port:** exactly as in Scenario A or E — the preserved IP, or a DHCP address if Fallback rescued it.
- **Port MAC:** the same MAC address.
- **Inside the guest:** this is where it differs, and it depends on the guest OS rather than on the source configuration.

**Guest outcome by OS**

| Guest OS | Source NIC was on DHCP | Source NIC was static |
| --- | --- | --- |
| Ubuntu 17.10 and newer | Converted to a static address — pinned to whatever address it happened to hold when discovered | Stays static on the same address |
| Ubuntu older than 17.10 | Stays on DHCP. Only interface-name rules are written | Stays static, per the guest's own configuration |
| RHEL family 6 and older | Legacy interface handling runs — verify after boot | Legacy interface handling runs — verify after boot |
| RHEL / CentOS / Rocky 7+, SUSE, other Linux | Nothing is written. The guest keeps its own configuration | Nothing is written. The guest keeps its own configuration |
| Windows | Nothing is written. The guest keeps its own configuration | Nothing is written. The guest keeps its own configuration |

On Ubuntu 17.10 and newer, the existing `/etc/netplan` directory is moved aside to `/etc/netplan-bkp` and replaced with a single generated file. Interfaces are renamed `vj0`, `vj1`, and so on, matched by MAC address. The netmask comes from what VMware Tools reported; if it is unknown, `/24` is assumed.

:::caution
On **Ubuntu 17.10 and newer** this rewrites the guest's networking wholesale. A NIC that was on DHCP comes back statically pinned to the address it held at discovery time. If you want it to stay on DHCP, use [Scenario C](#c--force-everything-onto-dhcp).

On **RHEL 7+, SUSE, other Linux, and Windows**, nothing is written at all. If the conversion renamed the interface, the guest's own configuration will point at a device that no longer exists. Have console access ready.

On a **multi-homed VM**, only one interface gets a default route, and which one is not predictable. Verify routing after migration.
:::

:::note
Whether the source NIC was on DHCP or static is recorded but never acted on directly by vJailbreak. It matters only because it determines what the guest's own untouched configuration says.
:::
