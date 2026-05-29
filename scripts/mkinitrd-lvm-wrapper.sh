#!/bin/sh
# vjailbreak: LVM /dev/<vg>/<lv> -> /dev/mapper/<vg>-<lv> translation wrapper
# Installed to fix SLES 11 mkinitrd when root is on an LVM logical volume.
_tmp=$(mktemp /tmp/vjailbreak-mkinitrd-args.XXXXXX)
trap 'rm -f "$_tmp"' EXIT
_skip=0
for _arg in "$@"; do
  if [ "$_skip" = "1" ]; then
    _skip=0
    case "$_arg" in
      /dev/*/*)
        _vg=$(printf '%s' "$_arg" | sed 's|^/dev/\([^/]*\)/.*|\1|')
        _lv=$(printf '%s' "$_arg" | sed 's|^/dev/[^/]*/||')
        _mapper="/dev/mapper/$(printf '%s' "${_vg}" | sed 's/-/--/g')-$(printf '%s' "${_lv}" | sed 's/-/--/g')"
        if [ -e "$_mapper" ]; then
          _arg="$_mapper"
        fi
        ;;
    esac
  fi
  case "$_arg" in -d) _skip=1 ;; esac
  printf '%s\0' "$_arg" >> "$_tmp"
done
xargs -0 /sbin/mkinitrd.orig < "$_tmp"
