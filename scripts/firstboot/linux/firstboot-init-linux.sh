#!/bin/bash
# Linux firstboot entry point for vjailbreak.
# Analogous to Firstboot-Init-Windows.bat on Windows.
#
# virt-v2v embeds this as a one-shot systemd/SysV service on the guest.
# It launches the scheduler from /linux-firstboot/ which was injected
# into the guest disk offline by InjectLinuxFirstBootScriptsFromStore.

exec /bin/bash /linux-firstboot/firstboot-scheduler.sh
