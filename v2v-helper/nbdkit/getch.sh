#!/bin/bash
# Download all fc44 RPMs needed for virt-v2v 2.11.8 upgrade
# Usage: ./fetch-fc44-rpms.sh [output-dir]
#
# Tries /updates/44/ first, falls back to /releases/44/ for each package.
# Compatible with both GNU grep (Linux) and BSD grep (macOS).
set -euo pipefail

OUTDIR="${1:-./fc44-rpms}"
BASE_UPDATES="https://www.rpmfind.net/linux/fedora/linux/updates/44/Everything/x86_64/Packages"
BASE_RELEASES="https://www.rpmfind.net/linux/fedora/linux/releases/44/Everything/x86_64/os/Packages"

mkdir -p "$OUTDIR"
cd "$OUTDIR"

echo "============================================================"
echo " Downloading fc44 RPMs for virt-v2v 2.11.8"
echo " Output: $(pwd)"
echo "============================================================"

# Helper: try updates first, then releases
wget_with_fallback() {
    local path="$1"
    local filename
    filename=$(basename "$path")

    if [[ -f "$filename" ]]; then
        echo "  [skip] $filename (already exists)"
        return 0
    fi

    echo -n "  [get]  $filename ... "

    # Try updates repo first
    if wget -q --tries=2 --timeout=20 "$BASE_UPDATES/$path" -O "$filename" 2>/dev/null; then
        echo "ok (updates)"
        return 0
    fi
    rm -f "$filename"

    # Fall back to releases repo
    if wget -q --tries=2 --timeout=20 "$BASE_RELEASES/$path" -O "$filename" 2>/dev/null; then
        echo "ok (releases)"
        return 0
    fi
    rm -f "$filename"

    echo "FAILED"
    return 1
}

FAILED=()
SUCCESS=0

# Helper to run wget_with_fallback and track result
get() {
    if wget_with_fallback "$1"; then
        ((SUCCESS++)) || true
    else
        FAILED+=("$1")
    fi
}

# ── virt-v2v ──────────────────────────────────────────────────────────────────
get "v/virt-v2v-2.11.8-1.fc44.x86_64.rpm"

# ── nbdkit stack (all same version — ABI must match exactly) ─────────────────
get "n/nbdkit-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-server-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-basic-plugins-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-basic-filters-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-vddk-plugin-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-ssh-plugin-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-curl-plugin-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-python-plugin-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-nbd-plugin-1.47.5-1.fc44.x86_64.rpm"
get "n/nbdkit-selinux-1.47.5-1.fc44.noarch.rpm"

# ── libnbd ────────────────────────────────────────────────────────────────────
get "l/libnbd-1.25.5-1.fc44.x86_64.rpm"
get "l/libnbd-devel-1.25.5-1.fc44.x86_64.rpm"

# ── libguestfs: discover actual version from index ───────────────────────────
echo -n "  [discover] libguestfs version ... "
LG_INDEX=$(wget -qO- --timeout=15 "$BASE_UPDATES/l/" 2>/dev/null || \
           wget -qO- --timeout=15 "$BASE_RELEASES/l/" 2>/dev/null || true)

LG_VER=$(echo "$LG_INDEX" \
    | grep -oE 'libguestfs-[0-9][0-9.]*-[0-9]+\.fc44\.x86_64\.rpm' \
    | grep -v "debuginfo\|debugsource\|devel\|appliance\|gobject\|java\|perl\|python\|ruby\|ocaml\|php\|lua\|xz\|rsync\|rescue\|tools\|winsupport\|javadoc\|bash" \
    | sort -t- -k2 -V | tail -1)

if [[ -n "$LG_VER" ]]; then
    echo "$LG_VER"
    get "l/$LG_VER"
    # appliance — same version number
    LG_APPLIANCE="${LG_VER/libguestfs-/libguestfs-appliance-}"
    get "l/$LG_APPLIANCE"
else
    echo "FAILED — could not discover libguestfs version"
    FAILED+=("l/libguestfs-?.fc44.x86_64.rpm")
    FAILED+=("l/libguestfs-appliance-?.fc44.x86_64.rpm")
fi

# ── guestfs-tools: discover version from index (BSD grep compatible) ─────────
echo ""
echo "  [discover] guestfs-tools version from index..."

# Try updates index first, then releases
GT_INDEX=$(wget -qO- --timeout=15 "$BASE_UPDATES/g/" 2>/dev/null || \
           wget -qO- --timeout=15 "$BASE_RELEASES/g/" 2>/dev/null || true)

# BSD-compatible: grep -oE instead of grep -oP
GT_RPM=$(echo "$GT_INDEX" \
    | grep -oE 'guestfs-tools-[0-9][^"< ]*\.fc44\.x86_64\.rpm' \
    | grep -v "debuginfo\|debugsource\|devel" \
    | sort -t- -k3 -V \
    | tail -1)

if [[ -n "$GT_RPM" ]]; then
    echo "  [found] $GT_RPM"
    # Figure out which base had the index
    if wget -q --tries=2 --timeout=20 "$BASE_UPDATES/g/$GT_RPM" -O "$GT_RPM" 2>/dev/null; then
        echo "  [ok]   $GT_RPM (updates)"
        ((SUCCESS++)) || true
    elif wget -q --tries=2 --timeout=20 "$BASE_RELEASES/g/$GT_RPM" -O "$GT_RPM" 2>/dev/null; then
        echo "  [ok]   $GT_RPM (releases)"
        ((SUCCESS++)) || true
    else
        rm -f "$GT_RPM"
        FAILED+=("g/$GT_RPM")
        echo "  [FAIL] $GT_RPM"
    fi
else
    echo "  [WARN] Could not auto-discover guestfs-tools fc44 RPM."
    echo "         Browse manually:"
    echo "           $BASE_UPDATES/g/"
    echo "           $BASE_RELEASES/g/"
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "============================================================"
echo " Downloaded: $SUCCESS packages"
echo ""
ls -lh ./*.rpm 2>/dev/null || echo " (no RPMs in directory)"

if [[ ${#FAILED[@]} -gt 0 ]]; then
    echo ""
    echo " FAILED (${#FAILED[@]}):"
    for f in "${FAILED[@]}"; do
        echo "   $BASE_UPDATES/$f"
        echo "   $BASE_RELEASES/$f"
    done
    echo ""
    echo " These packages may be at a different version in fc44."
    echo " Check the index pages above to find the actual filename."
    echo "============================================================"
    exit 1
fi
echo "============================================================"
