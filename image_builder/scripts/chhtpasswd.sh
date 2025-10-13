function chhtpasswd(){

set -euo pipefail

HTPASSWD_FILE="/etc/htpasswd"
USER_NAME="${1:-ubuntu}"

# Require sudo for writes; reading may be world-readable depending on perms.
if [[ ! -f "$HTPASSWD_FILE" ]]; then
  echo "Error: $HTPASSWD_FILE not found." >&2
  exit 1
fi

# Fetch existing hash for the user
existing_hash="$(awk -F: -v u="$USER_NAME" '$1==u{print $2}' "$HTPASSWD_FILE")"
if [[ -z "${existing_hash:-}" ]]; then
  echo "Error: user '$USER_NAME' not found in $HTPASSWD_FILE." >&2
  exit 1
fi

# Ensure it's an apr1 hash
if ! [[ "$existing_hash" == \$apr1\$* ]]; then
  echo "Error: existing hash for '$USER_NAME' is not an apr1 hash. Aborting." >&2
  exit 1
fi

# Prompt current password
read -s -p "Enter current password for '${USER_NAME}': " current_pw
echo

# Extract salt from $apr1$SALT$HASH
salt="$(echo "$existing_hash" | awk -F'$' '{print $3}')"
if [[ -z "${salt:-}" ]]; then
  echo "Error: could not parse salt from existing hash." >&2
  exit 1
fi

# Recompute apr1 hash using the same salt and compare
recalc_hash="$(openssl passwd -apr1 -salt "$salt" "$current_pw")"
if [[ "$recalc_hash" != "$existing_hash" ]]; then
  echo "Error: current password is incorrect." >&2
  exit 1
fi

counter=3
# Prompt for new password (twice)
while (($counter >  0)); do
  read -s -p "Enter new password: " new_pw
  echo
  if [[ ${#new_pw} -lt 8 ]]; then
          echo "Password too small";
          counter=$((counter -1));
          continue;
  fi
  read -s -p "Re-enter new password: " new_pw2
  echo
  if [[ -z "$new_pw" ]]; then
    echo "Error: new password cannot be empty." >&2
    counter=$(($counter - 1));
    continue
  fi
  if [[ "$new_pw" != "$new_pw2" ]]; then
    echo "Error: passwords do not match. Try again." >&2
    counter=$(($counter - 1));
    continue
  fi

  break
done
if [ "$counter" -eq 0 ];then
        echo "Error setting up the password";
        exit 1
fi
# Create new apr1 hash (random salt)
new_hash="$(openssl passwd -apr1 "$new_pw")"

# Safely update the file
tmpfile="$(mktemp "/tmp/htpasswd.${USER_NAME}.XXXXXX")"
trap 'rm -f "$tmpfile"' EXIT

awk -F: -v u="$USER_NAME" -v h="$new_hash" 'BEGIN{OFS=":"} $1==u{$2=h} {print}' "$HTPASSWD_FILE" > "$tmpfile"

# Preserve root ownership and 0644 as in your setup
sudo install -m 0644 -o root -g root "$tmpfile" "$HTPASSWD_FILE"

echo "Password for '$USER_NAME' updated successfully in $HTPASSWD_FILE."
}