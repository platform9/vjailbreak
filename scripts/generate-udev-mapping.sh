#!/bin/bash


# Source file containing MAC to IP mapping
NET_MAPPING_DATA="${NET_MAPPING_DATA:-/etc/macToIP}"

# Standard network configuration directories
RHEL_NET_DIR="${RHEL_NET_DIR:-/etc/sysconfig/network-scripts}"
SUSE_NET_DIR="${SUSE_NET_DIR:-/etc/sysconfig/network}"
NM_CONN_PATH="${NM_CONN_PATH:-/etc/NetworkManager/system-connections}"
NM_RUNTIME_DATA="${NM_RUNTIME_DATA:-/var/lib/NetworkManager}"
DHCP_LEASE_PATH="${DHCP_LEASE_PATH:-/var/lib/dhclient}"
DEBIAN_IF_DIR="${DEBIAN_IF_DIR:-/etc/network/interfaces}"
SYSTEMD_NET_PATH="${SYSTEMD_NET_PATH:-/run/systemd/network}"

# Tools and output targets
QUERY_TOOL="${QUERY_TOOL:-ifquery}"
UDEV_OUTPUT_TARGET="${UDEV_OUTPUT_TARGET:-/etc/udev/rules.d/70-persistent-net.rules}"
NETPLAN_BASE_DIR="${NETPLAN_BASE_DIR:-/}"
NETPLAN_EXT_CONF="${NETPLAN_EXT_CONF:-/etc/netplan/99-netcfg.yaml}"
USE_NETPLAN_LOGIC="${USE_NETPLAN_LOGIC:-true}"

# Setup custom file descriptor for logging to stdout
exec 3>&1
display_msg() {
    echo "$@" >&3
}

if [[ ! -f "$NET_MAPPING_DATA" ]]; then
    display_msg "Required mapping file $NET_MAPPING_DATA missing. Terminating."
    exit 0
fi

# Verify if the target udev file is already populated
if [[ -f "$UDEV_OUTPUT_TARGET" && -s "$UDEV_OUTPUT_TARGET" ]]; then
    display_msg "Target file $UDEV_OUTPUT_TARGET is already present and contains data. Terminating."
    exit 0
fi


# Strips whitespace and various quote marks from input strings
clean_string_input() {
    echo "$1" | tr -d '"' | tr -d "'" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

# Parses the mapping line to isolate MAC, IP, and device name components
# Format: MAC:ip:IP or MAC:ip:IP:dev:DEVICE
parse_address_pair() {
    FOUND_MAC=""
    FOUND_IP=""
    FOUND_DEV=""
    # Regex pattern to identify valid MAC:ip:IPv4 format
    if echo "$1" | grep -qE '^([0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}):ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}).*$'; then
        FOUND_MAC=$(echo "$1" | sed -nE 's/^([0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}):ip:.*$/\1/p')
        FOUND_IP=$(echo "$1" | sed -nE 's/^.*:ip:([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}).*$/\1/p')
        # Extract device name if present
        if echo "$1" | grep -q ':dev:'; then
            FOUND_DEV=$(echo "$1" | sed -nE 's/^.*:dev:([^:]+).*$/\1/p')
        fi
    fi
}


# Identifies the interface name from legacy ifcfg configuration files
resolve_device_via_ifcfg() {
    local CFG_PATH="$1"
    local TARGET_MAC="$2"

    # Priority 1: Direct DEVICE definition
    local DEV_ENTRY=$(grep '^DEVICE=' "$CFG_PATH" | cut -d'=' -f2)
    if [[ -n "$DEV_ENTRY" ]]; then
        echo "$DEV_ENTRY"
        return
    fi

    # Priority 2: Match via HWADDR entry
    local MAC_ENTRY=$(grep '^HWADDR=' "$CFG_PATH" | cut -d'=' -f2)
    if echo "$MAC_ENTRY" | grep -iq "$TARGET_MAC"; then
        # Pull name from filename suffix (e.g., ifcfg-eth0 -> eth0)
        echo "$(basename "$CFG_PATH" | awk -F'-' '{print $NF}')"
        return
    fi

    echo ""
}

# Processes ifcfg scripts for RHEL/SUSE style distributions
process_ifcfg_infrastructure() {
    local TARGET_DIR=""

    # Determine distribution type by directory presence
    if [[ -d "$RHEL_NET_DIR" ]]; then
        TARGET_DIR="$RHEL_NET_DIR"
    elif [[ -d "$SUSE_NET_DIR" ]]; then
        TARGET_DIR="$SUSE_NET_DIR"
    else
        display_msg "Notice: No standard ifcfg directory found."
        return 0
    fi

    # Track which MACs we've created rules for
    declare -A processed_macs
    local VJB_INDEX=1

    cat "$NET_MAPPING_DATA" | while read -r line_entry; do
        parse_address_pair "$line_entry"

        if [[ -z "$FOUND_MAC" || -z "$FOUND_IP" ]]; then
            display_msg "Skipping malformed mapping entry: $line_entry"
            continue
        fi

        # Skip if already processed
        if [[ -n "${processed_macs[$FOUND_MAC]}" ]]; then
            continue
        fi

        # Determine the device name to use
        local IF_NAME=""

        # Method 1: If macToIP has device name, use it directly
        if [[ -n "$FOUND_DEV" ]]; then
            IF_NAME="$FOUND_DEV"
            display_msg "Using device name from macToIP: $FOUND_MAC -> $IF_NAME"
        else
            # Method 2: Try to find ifcfg file by IP address (for static IPs)
            local CFG_FILE=$(grep -l "IPADDR=.*$FOUND_IP" "$TARGET_DIR"/ifcfg-* 2>/dev/null | head -1)
            if [[ -n "$CFG_FILE" ]]; then
                IF_NAME=$(resolve_device_via_ifcfg "$CFG_FILE" "$FOUND_MAC")
                if [[ -z "$IF_NAME" ]]; then
                    IF_NAME=$(basename "$CFG_FILE" | sed 's/^ifcfg-//')
                fi
                display_msg "Found device from ifcfg file (static IP): $FOUND_MAC -> $IF_NAME"
            else
                # Method 3: For DHCP interfaces or interfaces without static IP
                # Try to find by checking HWADDR in ifcfg files
                for IFCFG_FILE in "$TARGET_DIR"/ifcfg-*; do
                    [[ -f "$IFCFG_FILE" ]] || continue
                    
                    # Skip loopback
                    if [[ "$(basename "$IFCFG_FILE")" == "ifcfg-lo" ]]; then
                        continue
                    fi
                    
                    # Check if this file has HWADDR matching our MAC
                    if grep -qi "^HWADDR=.*$FOUND_MAC" "$IFCFG_FILE" 2>/dev/null; then
                        IF_NAME=$(basename "$IFCFG_FILE" | sed 's/^ifcfg-//')
                        display_msg "Found device from ifcfg file (HWADDR match): $FOUND_MAC -> $IF_NAME (IP: $FOUND_IP)"
                        break
                    fi
                done
                
                # Method 4: If still not found, try DHCP interfaces in order (best effort)
                if [[ -z "$IF_NAME" ]]; then
                    for IFCFG_FILE in "$TARGET_DIR"/ifcfg-*; do
                        [[ -f "$IFCFG_FILE" ]] || continue
                        
                        if [[ "$(basename "$IFCFG_FILE")" == "ifcfg-lo" ]]; then
                            continue
                        fi
                        
                        # Check if this is a DHCP interface without HWADDR
                        if grep -q "^BOOTPROTO=dhcp" "$IFCFG_FILE" 2>/dev/null && \
                           ! grep -q "^HWADDR=" "$IFCFG_FILE" 2>/dev/null; then
                            local POTENTIAL_DEVICE=$(basename "$IFCFG_FILE" | sed 's/^ifcfg-//')
                            
                            # Check if not already used
                            local DEVICE_USED=0
                            for used_mac in "${!processed_macs[@]}"; do
                                if [[ "${processed_macs[$used_mac]}" == "$POTENTIAL_DEVICE" ]]; then
                                    DEVICE_USED=1
                                    break
                                fi
                            done
                            
                            if [[ $DEVICE_USED -eq 0 ]]; then
                                IF_NAME="$POTENTIAL_DEVICE"
                                display_msg "Found device from ifcfg file (DHCP fallback): $FOUND_MAC -> $IF_NAME (IP: $FOUND_IP)"
                                break
                            fi
                        fi
                    done
                fi
            fi
        fi

        # If still no device name, create a new vjb interface
        if [[ -z "$IF_NAME" || "$IF_NAME" == "lo" ]]; then
            IF_NAME="vjb$VJB_INDEX"
            display_msg "Creating new vjb interface: $FOUND_MAC -> $IF_NAME"
            
            # Create ifcfg file
            {
                echo "TYPE=Ethernet"
                echo "BOOTPROTO=dhcp"
                echo "NAME=$IF_NAME"
                echo "DEVICE=$IF_NAME"
                echo "ONBOOT=yes"
                echo "HWADDR=$FOUND_MAC"
                echo "PEERDNS=yes"
                echo "PEERROUTES=yes"
                echo "DHCP_HOSTNAME=myhost"
            } > "$TARGET_DIR/ifcfg-$IF_NAME"
            VJB_INDEX=$((VJB_INDEX+1))
        fi

        # Create udev rule
        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(clean_string_input "$FOUND_MAC")\",NAME=\"$(clean_string_input "$IF_NAME")\""
        processed_macs["$FOUND_MAC"]="$IF_NAME"
    done
}

# Processes NetworkManager .nmconnection files
process_network_manager_files() {
    if [[ ! -d "$NM_CONN_PATH" ]]; then
        display_msg "Notice: NetworkManager path $NM_CONN_PATH not found."
        return 0
    fi

    cat "$NET_MAPPING_DATA" | while read -r line_entry; do
        parse_address_pair "$line_entry"

        if [[ -z "$FOUND_MAC" || -z "$FOUND_IP" ]]; then
            continue
        fi

        # Search connection files for the matching IP address
        local NM_SPECIFIC_FILE=$(grep -El "address[0-9]*=.*$FOUND_IP.*$" "$NM_CONN_PATH"/*)
        if [[ -z "$NM_SPECIFIC_FILE" ]]; then
            display_msg "Notice: No NM profile found for $FOUND_IP."
            continue
        fi

        local IF_NAME=$(grep '^interface-name=' "$NM_SPECIFIC_FILE" | cut -d'=' -f2)
        if [[ -z "$IF_NAME" ]]; then
            display_msg "Notice: Missing interface-name entry for $FOUND_IP."
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(clean_string_input "$FOUND_MAC")\",NAME=\"$(clean_string_input "$IF_NAME")\""
    done
}

# Extracts specific time metadata for NetworkManager UUIDs
fetch_uuid_time_marker() {
    local STAMP_DB="$NM_RUNTIME_DATA/timestamps"

    if [[ ! -f "$STAMP_DB" ]]; then
        display_msg "Warning: Timestamp database $STAMP_DB missing."
        echo ""
        return
    fi

    while IFS='=' read -r KEY_ID VAL_TIME; do
        # Ignore ini headers and invalid lines
        [[ "$KEY_ID" == "[timestamps]" ]] && continue
        [[ -z "$KEY_ID" || -z "$VAL_TIME" ]] && continue

        if [[ "$KEY_ID" == "$1" ]]; then
            echo "$VAL_TIME"
            break
        fi
    done < "$STAMP_DB"
}

# Cross-references NM DHCP leases to find interface names
process_nm_leases() {
    if [[ ! -d "$NM_RUNTIME_DATA" ]]; then
        display_msg "Notice: $NM_RUNTIME_DATA does not exist."
        return 0
    fi

    while read -r line_entry; do
        parse_address_pair "$line_entry"

        if [[ -z "$FOUND_MAC" || -z "$FOUND_IP" ]]; then
            continue
        fi

        local LEASE_MATCHES=$(grep -El "ADDRESS=$FOUND_IP$" "$NM_RUNTIME_DATA"/*.lease)
        if [[ -z "$LEASE_MATCHES" ]]; then
            display_msg "Notice: No NM leases found for $FOUND_IP"
            continue
        fi

        # Logic to find the most recent lease based on UUID/Timestamp
        local IF_NAME=$(for L_FILE in $LEASE_MATCHES; do
            display_msg "Analyzing lease: $L_FILE"
            # Format: ...-UUID-INTERFACE.lease
            local PARTS=$(echo "$L_FILE" | sed -n 's|^.*-\([0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}\)-\(.*\)\.lease$|\1 \2|p')
            if [[ -n "$PARTS" ]]; then
                local UUID_STR=$(echo "$PARTS" | cut -d' ' -f1)
                local INT_STR=$(echo "$PARTS" | cut -d' ' -f2)
                local T_STAMP=$(fetch_uuid_time_marker "$UUID_STR")
                
                if [[ -n "$T_STAMP" ]]; then
                    echo "$T_STAMP $INT_STR"
                else
                    echo "0 $INT_STR"
                fi
            fi
        done | sort -nr | head -1 | cut -d' ' -f2)

        if [[ -z "$IF_NAME" ]]; then
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(clean_string_input "$FOUND_MAC")\",NAME=\"$(clean_string_input "$IF_NAME")\""
    done < "$NET_MAPPING_DATA"
}

# Processes traditional dhclient lease files by parsing block syntax
process_dhclient_history() {
    if [[ ! -d "$DHCP_LEASE_PATH" ]]; then
        display_msg "Notice: $DHCP_LEASE_PATH not available."
        return 0
    fi

    while read -r line_entry; do
        parse_address_pair "$line_entry"

        if [[ -z "$FOUND_MAC" || -z "$FOUND_IP" ]]; then
            continue
        fi

        local WINNING_EPOCH=0
        local FINAL_IF=""

        local ACTIVE_IF=""
        local ACTIVE_IP=""
        local ACTIVE_EXP=""

        for LEASE_DB in "$DHCP_LEASE_PATH"/*; do
            while IFS= read -r raw_line || [[ -n "$raw_line" ]]; do
                # Strip spaces and closing semicolons
                local cleaned_line=$(echo "$raw_line" | sed -e 's/^[[:space:]]*//' -e 's/;[[:space:]]*$//')
                
                case "$cleaned_line" in
                    'interface'*)
                        ACTIVE_IF=$(echo "$cleaned_line" | sed -n 's/.*"\(.*\)".*/\1/p')
                        ;;
                    'expire'*)
                        ACTIVE_EXP=$(echo "$cleaned_line" | awk '{print $3, $4}')
                        ;;
                    'fixed-address'*)
                        ACTIVE_IP=$(echo "$cleaned_line" | awk '{print $2}')
                        ;;
                    '}')
                        if [[ -n "$ACTIVE_IP" && -n "$ACTIVE_IF" && -n "$ACTIVE_EXP" ]]; then
                            if [[ "$FOUND_IP" == "$ACTIVE_IP" ]]; then
                                local current_epoch=$(date -d "$ACTIVE_EXP" +%s 2>/dev/null)
                                if [[ -n "$current_epoch" && "$current_epoch" -gt "$WINNING_EPOCH" ]]; then
                                    WINNING_EPOCH=$current_epoch
                                    FINAL_IF=$ACTIVE_IF
                                fi
                            fi
                        fi
                        # Reset block variables
                        ACTIVE_IP=""
                        ACTIVE_IF=""
                        ACTIVE_EXP=""
                        ;;
                esac
            done < "$LEASE_DB"
        done

        if [[ -z "$FINAL_IF" ]]; then
            display_msg "Notice: No dhclient lease found for $FOUND_IP"
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(clean_string_input "$FOUND_MAC")\",NAME=\"$(clean_string_input "$FINAL_IF")\""
    done < "$NET_MAPPING_DATA"
}

# --- Modern Network Management (Netplan) ---

process_netplan_logic() {
    if [[ "$USE_NETPLAN_LOGIC" == "false" ]]; then
        return 1
    fi
    
    if ! ${IN_TEST_MODE:-false} && ! command -v netplan >/dev/null 2>&1; then
        display_msg "Notice: Netplan tool not present."
        return 0
    fi

    # Detect if 'netplan get' is available for easy parsing
    has_netplan_get_feature() {
        if ${SKIP_NETPLAN_GET:-false}; then return 1; fi
        netplan get >&3
        return $?
    }

    invoke_netplan_get() {
        netplan get --root-dir "$NETPLAN_BASE_DIR" "$@" 2>&3
    }

    # Internal helper to map IP to Netplan interface name
    locate_netplan_if_by_ip() {
        local search_ip="$1"
        if has_netplan_get_feature; then
            invoke_netplan_get ethernets | grep -Eo "^[^[:space:]]+[^:]" | while read -r INTERFACE; do
                if invoke_netplan_get ethernets."$INTERFACE".addresses | grep -q "$search_ip"; then
                    echo "$INTERFACE"
                    return
                fi
            done
        else
            if [[ -z "$SYSTEMD_NET_PATH" ]]; then return; fi
            netplan generate --root-dir "$NETPLAN_BASE_DIR" 2>&3
            local MATCHING_FILE=$(grep -El "Address[0-9]*=.*$FOUND_IP.*$" "$SYSTEMD_NET_PATH"/*)
            [[ -z "$MATCHING_FILE" ]] && return
            grep '^Name=' "$MATCHING_FILE" | cut -d'=' -f2
        fi
    }

    local GEN_NETPLAN_TMP="/tmp/netplan_output.yaml"
    {
        echo "network:"
        echo "  version: 2"
        echo "  renderer: networkd"
        echo "  ethernets:"
    } > "$GEN_NETPLAN_TMP"

    local NETPLAN_ID=1
    local DID_INJECT=0

    while read -r line_entry; do
        parse_address_pair "$line_entry"

        if [[ -z "$FOUND_MAC" || -z "$FOUND_IP" ]]; then
            continue
        fi

        local IF_NAME=$(locate_netplan_if_by_ip "$FOUND_IP")

        if [[ -z "$IF_NAME" ]]; then
            # Inject dynamic interface if not found in existing netplan
            {
                echo "    vjb$NETPLAN_ID:"
                echo "      match:"
                echo "        macaddress: $FOUND_MAC"
                echo "      dhcp4: true"
            } >> "$GEN_NETPLAN_TMP"

            NETPLAN_ID=$((NETPLAN_ID+1))
            DID_INJECT=1
            continue
        fi

        echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(clean_string_input "$FOUND_MAC")\",NAME=\"$(clean_string_input "$IF_NAME")\""
    done < "$NET_MAPPING_DATA"

    if [[ "$DID_INJECT" -eq 1 ]]; then
        display_msg "Status: Injecting wildcard configurations into Netplan."
        cp "$GEN_NETPLAN_TMP" "$NETPLAN_EXT_CONF"
    fi
    
    rm -f "$GEN_NETPLAN_TMP"
}

# Processes interfaces via the ifquery utility
process_ifquery_infrastructure() {
    if ! ${IN_TEST_MODE:-false} && ! command -v "$QUERY_TOOL" >/dev/null 2>&1; then
        display_msg "Notice: Tool $QUERY_TOOL not found."
        return 0
    fi

    # Helper to call ifquery against the specific interfaces dir
    invoke_ifquery() {
        "$QUERY_TOOL" -i "$DEBIAN_IF_DIR" "$@" 2>&3
    }

    find_if_matching_ip() {
        local search_ip="$1"
        invoke_ifquery -l | while read -r INTERFACE; do
            if invoke_ifquery "$INTERFACE" | grep -q "$search_ip"; then
                echo "$INTERFACE"
                return
            fi
        done
    }

    cat "$NET_MAPPING_DATA" | while read -r line_entry; do
        parse_address_pair "$line_entry"

        if [[ -z "$FOUND_MAC" || -z "$FOUND_IP" ]]; then
            continue
        fi

        local IF_NAME=$(find_if_matching_ip "$FOUND_IP")

        if [[ -n "$IF_NAME" ]]; then
             echo "SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"$(clean_string_input "$FOUND_MAC")\",NAME=\"$(clean_string_input "$IF_NAME")\""
        fi
    done
}


# Filters out any duplicate hardware addresses before writing
validate_hardware_uniqueness() {
    local RAW_CONTENT=$(cat)
    # Isolate MACs, standardize casing, and look for repeats
    local DUPLICATE_LIST=$(echo "$RAW_CONTENT" | grep -ioE "[0-9A-F:]{17}" | tr 'a-f' 'A-F' | sort | uniq -d)

    if [[ -n "$DUPLICATE_LIST" ]]; then
        display_msg "Warning: Detected redundant MAC addresses: $DUPLICATE_LIST"
        return 0
    fi

    echo "$RAW_CONTENT"
}

# Orchestrates the discovery modules and finalizes udev rules
execute_main_workflow() {
    {
        process_ifcfg_infrastructure
        process_network_manager_files
        process_nm_leases
        process_dhclient_history
        process_netplan_logic
        process_ifquery_infrastructure
    } | validate_hardware_uniqueness > "$UDEV_OUTPUT_TARGET" 2>/dev/null

    display_msg "Generated udev rules successfully:"
    cat "$UDEV_OUTPUT_TARGET"
}

# Launch
execute_main_workflow