set -euo pipefail

DISK="${1:-${DISK:-}}"

if [[ -z "$DISK" ]]; then
    echo "Usage: $0 /path/to/disk.img" >&2
    exit 1
fi

if [[ ! -e "$DISK" ]]; then
    echo "ERROR: disk not found: $DISK" >&2
    exit 1
fi

echo "Target disk: $DISK"

guestfish -a "$DISK" -i --rw <<'EOF'

-rm-rf "/Program Files/VMware/VMware Tools"
-rm-rf "/Program Files/VMware"
-rm-rf "/Program Files (x86)/VMware"

-rm-rf "/Program Files/Common Files/VMware"

-rm-rf "/ProgramData/VMware"

-rm-rf "/ProgramData/Microsoft/Windows/Start Menu/Programs/VMware"

-rm-rf "/Users/Administrator/AppData/Local/VMware"
-rm-rf "/Users/Administrator/AppData/Roaming/VMware"
-rm-rf "/Users/Default/AppData/Local/VMware"
-rm-rf "/Users/Default/AppData/Roaming/VMware"

-rm-f "/Windows/System32/drivers/vmci.sys"
-rm-f "/Windows/System32/drivers/vm3dmp.sys"
-rm-f "/Windows/System32/drivers/vm3dmp_loader.sys"
-rm-f "/Windows/System32/drivers/vm3dmp-debug.sys"
-rm-f "/Windows/System32/drivers/vm3dmp-stats.sys"
-rm-f "/Windows/System32/drivers/vmaudio.sys"
-rm-f "/Windows/System32/drivers/vmhgfs.sys"
-rm-f "/Windows/System32/drivers/vmmemctl.sys"
-rm-f "/Windows/System32/drivers/vmmouse.sys"
-rm-f "/Windows/System32/drivers/vmrawdsk.sys"
-rm-f "/Windows/System32/drivers/vmtools.sys"
-rm-f "/Windows/System32/drivers/vmusbmouse.sys"
-rm-f "/Windows/System32/drivers/vmvss.sys"
-rm-f "/Windows/System32/drivers/vsock.sys"
-rm-f "/Windows/System32/drivers/vmx_svga.sys"
-rm-f "/Windows/System32/drivers/vmxnet3.sys"
-rm-f "/Windows/System32/drivers/vmgencounter.sys"
-rm-f "/Windows/System32/drivers/vmgid.sys"
-rm-f "/Windows/System32/drivers/vms3cap.sys"
-rm-f "/Windows/System32/drivers/vmstorfl.sys"
-rm-f "/Windows/System32/drivers/vmscsi.sys"

EOF

echo "Filesystem cleanup done."

if command -v virt-win-reg &>/dev/null; then
    echo "Removing VMware registry keys offline..."

    REG_TMP=$(mktemp /tmp/vmware-del-XXXXXX.reg)
    cat >"$REG_TMP" <<'REGEOF'
Windows Registry Editor Version 5.00

[-HKEY_LOCAL_MACHINE\SOFTWARE\VMware, Inc.]
[-HKEY_LOCAL_MACHINE\SOFTWARE\WOW6432Node\VMware, Inc.]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VMTools]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VMMemCtl]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmci]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vm3dmp]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmaudio]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmhgfs]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmmouse]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmrawdsk]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmusbmouse]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmvss]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vsock]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmxnet3]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VMRawDisk]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vmrawdsk]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vm3dservice]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\vnetWFP]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VMwareCAF]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VMwareCAFCommAmqpListener]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VMwareCAFManagementAgentHost]
[-HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\VGAuthService]
REGEOF

    virt-win-reg --merge "$DISK" "$REG_TMP" && \
        echo "Registry keys removed." || \
        echo "WARN: virt-win-reg merge failed (non-fatal)." >&2

    rm -f "$REG_TMP"
else
    echo "virt-win-reg not found; skipping offline registry cleanup."
    echo "  Install libguestfs-winsupport for registry hive editing, or let"
fi

echo "offline vmware cleanup complete"
