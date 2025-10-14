
HTPASSWD_FILE="/etc/htpasswd"
DEFAULT_USER="ubuntu"

usage() {
  cat >&2 <<USAGE
Usage:
  pf9-htpasswd create-user <username>
  pf9-htpasswd update-password <username>
  pf9-htpasswd delete-user <username>

Notes:
  - Stores/reads entries in $HTPASSWD_FILE as username:$apr1$salt$hash (openssl apr1)
  - You may be prompted for passwords and confirmation.
USAGE
}

ensure_file() {
  if [[ ! -f "$HTPASSWD_FILE" ]]; then
    echo "Error: $HTPASSWD_FILE not found." >&2
    return 1
  fi
}

user_exists() {
  local u="$1"
  awk -F: -v u="$u" '$1==u{found=1} END{exit !found}' "$HTPASSWD_FILE"
}

get_hash() {
  local u="$1"
  awk -F: -v u="$u" '$1==u{print $2}' "$HTPASSWD_FILE"
}

prompt_user() {
  local def="${1:-$DEFAULT_USER}"
  local __varname="$2"
  local input
  read -p "Enter username [${def}]: " input || true
  if [[ -z "${input:-}" ]]; then
    printf -v "$__varname" '%s' "$def"
  else
    printf -v "$__varname" '%s' "$input"
  fi
}

prompt_new_password() {
  local __out="$1"
  local counter=3
  local new_pw new_pw2
  while ((counter>0)); do
    read -s -p "Enter new password: " new_pw; echo
    if [[ ${#new_pw} -lt 8 ]]; then
      echo "Password too small" >&2
      ((counter--))
      continue
    fi
    read -s -p "Re-enter new password: " new_pw2; echo
    if [[ -z "$new_pw" ]]; then
      echo "Error: new password cannot be empty." >&2
      ((counter--))
      continue
    fi
    if [[ "$new_pw" != "$new_pw2" ]]; then
      echo "Error: passwords do not match. Try again." >&2
      ((counter--))
      continue
    fi
    break
  done
  if ((counter==0)); then
    echo "Error setting up the password" >&2
    return 1
  fi
  printf -v "$__out" '%s' "$new_pw"
}

create_user() {
  local user="$1"
  ensure_file
  if user_exists "$user"; then
    echo "Error: user '$user' already exists in $HTPASSWD_FILE." >&2
    return 1
  fi
  local pw
  prompt_new_password pw
  local new_hash
  new_hash="$(openssl passwd -apr1 "$pw")"
  local tmpfile
  tmpfile="$(mktemp "/tmp/htpasswd.${user}.XXXXXX")"
  trap 'rm -f "$tmpfile"' EXIT
  cat "$HTPASSWD_FILE" > "$tmpfile"
  printf "%s:%s\n" "$user" "$new_hash" >> "$tmpfile"
  sudo install -m 0644 -o root -g root "$tmpfile" "$HTPASSWD_FILE"
  echo "User '$user' added to $HTPASSWD_FILE."
}

change_password() {
  local user="$1"
  ensure_file
  local existing_hash
  existing_hash="$(get_hash "$user")"
  if [[ -z "${existing_hash:-}" ]]; then
    echo "Error: user '$user' not found in $HTPASSWD_FILE." >&2
    return 1
  fi
  if ! [[ "$existing_hash" == \$apr1\$* ]]; then
    echo "Error: existing hash for '$user' is not an apr1 hash. Aborting." >&2
    return 1
  fi
  read -s -p "Enter current password for '${user}': " current_pw; echo
  local salt recalc_hash
  salt="$(echo "$existing_hash" | awk -F'$' '{print $3}')"
  if [[ -z "${salt:-}" ]]; then
    echo "Error: could not parse salt from existing hash." >&2
    return 1
  fi
  recalc_hash="$(openssl passwd -apr1 -salt "$salt" "$current_pw")"
  if [[ "$recalc_hash" != "$existing_hash" ]]; then
    echo "Error: current password is incorrect." >&2
    return 1
  fi
  local pw
  prompt_new_password pw
  local new_hash tmpfile
  new_hash="$(openssl passwd -apr1 "$pw")"
  tmpfile="$(mktemp "/tmp/htpasswd.${user}.XXXXXX")"
  trap 'rm -f "$tmpfile"' EXIT
  awk -F: -v u="$user" -v h="$new_hash" 'BEGIN{OFS=":"} $1==u{$2=h} {print}' "$HTPASSWD_FILE" > "$tmpfile"
  sudo install -m 0644 -o root -g root "$tmpfile" "$HTPASSWD_FILE"
  echo "Password for '$user' updated successfully in $HTPASSWD_FILE."
}

delete_user() {
  local user="$1"
  ensure_file
  local existing_hash
  existing_hash="$(get_hash "$user")"
  if [[ -z "${existing_hash:-}" ]]; then
    echo "Error: user '$user' not found in $HTPASSWD_FILE." >&2
    return 1
  fi
  local __conf
  read -p "Delete user '${user}' from $HTPASSWD_FILE? [y/N]: " __conf
  case "${__conf:-}" in
    y|Y|yes|YES) ;;
    *) echo "Aborted."; return 1;;
  esac
  local tmpfile
  tmpfile="$(mktemp "/tmp/htpasswd.${user}.XXXXXX")"
  trap 'rm -f "$tmpfile"' EXIT
  awk -F: -v u="$user" 'BEGIN{OFS=":"} $1!=u{print}' "$HTPASSWD_FILE" > "$tmpfile"
  sudo install -m 0644 -o root -g root "$tmpfile" "$HTPASSWD_FILE"
  echo "User '$user' removed from $HTPASSWD_FILE."
}

_pf9_ht_main() {
  local cmd="${1:-}"; shift || true
  case "$cmd" in
    create-user)
      local user="${1:-}"; shift || true
      if [[ -z "${user:-}" ]]; then
        prompt_user "$DEFAULT_USER" user
      fi
      create_user "$user"
      ;;
    update-password)
      local user="${1:-}"; shift || true
      if [[ -z "${user:-}" ]]; then
        prompt_user "$DEFAULT_USER" user
      fi
      change_password "$user"
      ;;
    delete-user)
      local user="${1:-}"; shift || true
      if [[ -z "${user:-}" ]]; then
        prompt_user "$DEFAULT_USER" user
      fi
      delete_user "$user"
      ;;
    -h|--help|help|"")
      usage; return 2
      ;;
    *)
      echo "Unknown command: $cmd" >&2
      usage; return 2
      ;;
  esac
}

# Define shell function to use as a command when sourced in .bashrc
pf9_htpasswd() {
  _pf9_ht_main "$@"
}

# Alias with hyphenated name for convenience
alias pf9-htpasswd='pf9_htpasswd'
