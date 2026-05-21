#!/bin/bash
# Linux Firstboot Scheduler for vjailbreak
# Mirrors Firstboot-Scheduler.ps1 behavior exactly:
#
#   - Reads scripts.json on first run, creates state file
#   - State file: "ScriptName|async|runcount"  (-1 = not yet run)
#   - Registers itself as a boot service BEFORE running any script
#     (so a reboot mid-run resumes from where it left off)
#   - Script-Runner: exponential backoff, max 3 attempts (60→120→300s)
#   - Async=false failure: break loop, reschedule on next boot
#   - Async=true  failure: log warning, continue
#   - All scripts done: unregister boot service, exit 0
#
# Failure semantics (identical to Windows):
#   Async=false (sync):  failure stops scheduler; rescheduled on next boot
#   Async=true:          failure logged as warning; next script still runs

SCRIPTS_DIR="/linux-firstboot"
LOG_FILE="/var/log/firstboot-scheduler.log"
STATE_FILE="${SCRIPTS_DIR}/firstboot-scheduler.state"
SCRIPTS_JSON="${SCRIPTS_DIR}/scripts.json"
SCHEDULER_SCRIPT="${SCRIPTS_DIR}/firstboot-scheduler.sh"
SERVICE_NAME="vjailbreak-firstboot"
MAX_RETRIES=3
RETRY_WAIT=60
RETRY_CAP=300

# ----------------------------------------------------------------------------
# Logging
# ----------------------------------------------------------------------------
log() {
    local level="$1"
    local msg="$2"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $msg" | tee -a "$LOG_FILE"
}

# ----------------------------------------------------------------------------
# schedule_self / remove_self
# Mirror of Schedule-MyTask / Remove-MyTask in the Windows scheduler.
# Registers this script as a startup service so a reboot resumes the run.
# ----------------------------------------------------------------------------
schedule_self() {
    if command -v systemctl >/dev/null 2>&1 && systemctl --version >/dev/null 2>&1; then
        cat >/etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=vjailbreak Linux Firstboot Scheduler
After=network.target
ConditionPathExists=${SCRIPTS_JSON}

[Service]
Type=oneshot
ExecStart=/bin/bash ${SCHEDULER_SCRIPT}
RemainAfterExit=no
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload
        systemctl enable "${SERVICE_NAME}.service" 2>>"$LOG_FILE"
        log "INFO" "Registered startup service ${SERVICE_NAME} (systemd)"
    elif command -v chkconfig >/dev/null 2>&1; then
        # SysV fallback for SUSE 11.x / older RHEL
        local init_script="/etc/init.d/${SERVICE_NAME}"
        cat >"$init_script" <<EOF
#!/bin/bash
### BEGIN INIT INFO
# Provides:          ${SERVICE_NAME}
# Required-Start:    \$network
# Required-Stop:
# Default-Start:     2 3 4 5
# Default-Stop:
# Short-Description: vjailbreak firstboot scheduler
### END INIT INFO
exec /bin/bash ${SCHEDULER_SCRIPT}
EOF
        chmod +x "$init_script"
        chkconfig --add "${SERVICE_NAME}" 2>>"$LOG_FILE"
        chkconfig "${SERVICE_NAME}" on 2>>"$LOG_FILE"
        log "INFO" "Registered startup service ${SERVICE_NAME} (SysV)"
    else
        log "WARNING" "No init system detected — cannot register for cross-reboot resume"
    fi
}

remove_self() {
    if command -v systemctl >/dev/null 2>&1 && systemctl --version >/dev/null 2>&1; then
        systemctl disable "${SERVICE_NAME}.service" 2>>"$LOG_FILE" || true
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload 2>>"$LOG_FILE" || true
        log "INFO" "Unregistered startup service ${SERVICE_NAME} (systemd)"
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig "${SERVICE_NAME}" off 2>>"$LOG_FILE" || true
        chkconfig --del "${SERVICE_NAME}" 2>>"$LOG_FILE" || true
        rm -f "/etc/init.d/${SERVICE_NAME}"
        log "INFO" "Unregistered startup service ${SERVICE_NAME} (SysV)"
    fi
}

# ----------------------------------------------------------------------------
# init_table
# Mirror of Init-Table: read scripts.json, create state file.
# State format: "ScriptName|async|runcount"   (-1 = not yet run)
# ----------------------------------------------------------------------------
init_table() {
    if [ ! -f "$SCRIPTS_JSON" ]; then
        log "ERROR" "scripts.json not found: $SCRIPTS_JSON"
        return 1
    fi

    # Parse JSON without jq — bash 3.x compatible via temp file
    local tmpfile
    tmpfile=$(mktemp)
    tr -d '[]{}' <"$SCRIPTS_JSON" | tr ',' '\n' | sed 's/^[[:space:]]*//' >"$tmpfile"

    local script="" async=""
    >"$STATE_FILE"
    while IFS= read -r line; do
        case "$line" in
            *'"Script"'*)
                script=$(echo "$line" | sed 's/.*"Script"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
                ;;
            *'"Async"'*)
                async=$(echo "$line" | sed 's/.*"Async"[[:space:]]*:[[:space:]]*\([a-z]*\).*/\1/')
                ;;
        esac
        if [ -n "$script" ] && [ -n "$async" ]; then
            echo "${script}|${async}|-1" >>"$STATE_FILE"
            script=""
            async=""
        fi
    done <"$tmpfile"
    rm -f "$tmpfile"

    log "INFO" "State file initialised from scripts.json"
}

# ----------------------------------------------------------------------------
# get_script
# Mirror of Get-Script: returns next script whose runcount is -1 or 0–2
# and is not in the failed-scripts list (passed as remaining args).
# Outputs "script_name async_value" on stdout, empty if none left.
# ----------------------------------------------------------------------------
get_script() {
    # Remaining args are already-failed script names
    # Build a lookup: failed_<name>=1
    local failed_lookup=""
    while [ "$#" -gt 0 ]; do
        failed_lookup="${failed_lookup}:${1}:"
        shift
    done

    while IFS='|' read -r name async runcount; do
        [ -z "$name" ] && continue
        # Skip if in failed list
        case "$failed_lookup" in
            *":${name}:"*) continue ;;
        esac
        # runcount -1 = not yet run; 0-2 = in progress (resume after reboot)
        if [ "$runcount" -eq -1 ] || { [ "$runcount" -ge 0 ] && [ "$runcount" -lt "$MAX_RETRIES" ]; }; then
            echo "${name} ${async}"
            return 0
        fi
    done <"$STATE_FILE"
    # Nothing left
    echo ""
}

# ----------------------------------------------------------------------------
# push_script
# Mirror of Push-Script: increment runcount; throw if already at max.
# ----------------------------------------------------------------------------
push_script() {
    local target="$1"
    local tmpfile
    tmpfile=$(mktemp)

    local found=false
    while IFS='|' read -r name async runcount; do
        if [ "$name" = "$target" ]; then
            if [ "$runcount" -ge "$MAX_RETRIES" ]; then
                rm -f "$tmpfile"
                log "ERROR" "Script '$target' has reached its maximum run times (${MAX_RETRIES})."
                return 1
            fi
            runcount=$((runcount + 1))
            found=true
        fi
        echo "${name}|${async}|${runcount}" >>"$tmpfile"
    done <"$STATE_FILE"

    if ! $found; then
        rm -f "$tmpfile"
        log "ERROR" "Script '$target' not found in state file."
        return 1
    fi

    mv "$tmpfile" "$STATE_FILE"
}

# ----------------------------------------------------------------------------
# pop_script
# Mirror of Pop-Script: remove script from state file on success.
# ----------------------------------------------------------------------------
pop_script() {
    local target="$1"
    local tmpfile
    tmpfile=$(mktemp)
    grep -v "^${target}|" "$STATE_FILE" >"$tmpfile" || true
    mv "$tmpfile" "$STATE_FILE"
}

# ----------------------------------------------------------------------------
# script_runner
# Mirror of Script-Runner: exponential backoff, max MAX_RETRIES attempts.
# Returns 0 on success, 1 on exhausted retries.
# ----------------------------------------------------------------------------
script_runner() {
    local script_path="$1"
    local attempt=0
    local wait_time=$RETRY_WAIT

    if [ ! -f "$script_path" ]; then
        log "ERROR" "Script file not found: $script_path"
        return 1
    fi

    chmod +x "$script_path"

    while [ "$attempt" -lt "$MAX_RETRIES" ]; do
        attempt=$((attempt + 1))
        log "INFO" "Executing $script_path (attempt ${attempt}/${MAX_RETRIES})"

        if bash "$script_path" >>"$LOG_FILE" 2>&1; then
            log "INFO" "Script executed successfully"
            return 0
        fi

        log "ERROR" "attempt (${attempt}): Script failed with non-zero exit code"

        if [ "$attempt" -lt "$MAX_RETRIES" ]; then
            log "INFO" "Retrying in ${wait_time}s..."
            sleep "$wait_time"
            wait_time=$((wait_time * 2))
            if [ "$wait_time" -gt "$RETRY_CAP" ]; then
                wait_time=$RETRY_CAP
            fi
        fi
    done

    return 1
}

# ----------------------------------------------------------------------------
# Main — mirrors the main try{} block in Firstboot-Scheduler.ps1
# ----------------------------------------------------------------------------
main() {
    log "INFO" "=== Starting Firstboot Scheduler ==="
    log "INFO" "Script root: ${SCRIPTS_DIR}"

    if [ ! -f "$SCRIPTS_JSON" ]; then
        log "WARNING" "scripts.json not found at: $SCRIPTS_JSON"
        exit 0
    fi

    log "INFO" "Found scripts.json at: $SCRIPTS_JSON"

    # Register for next boot BEFORE running anything (mirrors line 326 in .ps1)
    schedule_self

    # Initialise state file on first run
    if [ ! -f "$STATE_FILE" ]; then
        log "INFO" "State file does not exist, creating..."
        if ! init_table; then
            log "ERROR" "Failed to initialise state table"
            exit 1
        fi
    fi

    # Track failed async scripts (mirrors $failedScriptNames list in .ps1)
    failed_scripts=""

    # Main loop — mirrors while($true) in .ps1
    while true; do
        # Build arg list for get_script from colon-separated failed list
        local next_entry
        if [ -n "$failed_scripts" ]; then
            # shellcheck disable=SC2086
            next_entry=$(get_script $failed_scripts)
        else
            next_entry=$(get_script)
        fi

        if [ -z "$next_entry" ]; then
            log "INFO" "No scripts to run, exiting..."
            remove_self
            break
        fi

        local script async
        script=$(echo "$next_entry" | cut -d' ' -f1)
        async=$(echo "$next_entry" | cut -d' ' -f2)
        log "INFO" "Selected script: $script (async: $async)"

        if ! push_script "$script"; then
            log "ERROR" "push_script failed for $script — aborting"
            break
        fi

        if script_runner "${SCRIPTS_DIR}/${script}"; then
            log "INFO" "Script '$script' executed successfully"
            pop_script "$script"
        else
            log "ERROR" "Script '$script' failed"
            failed_scripts="${failed_scripts} ${script}"

            if [ "$async" = "false" ]; then
                log "ERROR" "Script failed — breaking to reschedule on reboot"
                break
            else
                log "WARNING" "Continuing to next script"
            fi
        fi
    done

    if [ -n "$failed_scripts" ]; then
        log "WARNING" "SUMMARY: failed scripts (async, scheduler continued):${failed_scripts}"
    fi

    log "INFO" "=== Firstboot Scheduler completed ==="
    exit 0
}

main
