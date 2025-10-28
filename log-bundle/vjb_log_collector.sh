#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

LOG_DIR="/var/log/pf9"
OUTPUT_DIR="/tmp/vjb-support-bundle-$(date +%Y%m%d-%H%M%S)"
NAMESPACE="migration-system"
DATE_FILTER=""
VM_FILTER=""
INCLUDE_DEBUG=false

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Collect vJailbreak logs and system information for support bundle.

OPTIONS:
    --vms <vm1,vm2,vm3>     Filter logs for specific VMs (comma-separated)
    --date <timestamp>      Filter logs from specific date (Unix timestamp or YYYY-MM-DD)
    --debug                 Include debug logs
    --output-dir <path>     Custom output directory (default: /tmp/vjb-support-bundle-<timestamp>)
    --namespace <name>      Kubernetes namespace (default: pf9-vjb)
    -h, --help              Show this help message

EXAMPLES:
    $0 --vms migration-gk-vm-10,migration-gk-vm-20
    $0 --date 2025-10-27
    $0 --date 1730000000 --debug
    $0 --vms vm1,vm2 --date 2025-10-27 --debug

EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --vms)
            VM_FILTER="$2"
            shift 2
            ;;
        --date)
            DATE_FILTER="$2"
            shift 2
            ;;
        --debug)
            INCLUDE_DEBUG=true
shift
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

print_info "Creating support bundle directory: $OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"/{logs,versions,configs,system-info}

collect_controller_logs() {
    print_info "Collecting controller logs"
    if command -v kubectl &> /dev/null; then
	local pods=$(sudo kubectl get pods -n "$NAMESPACE" -o name | grep migration-controller | cut -d'/' -f2)
        if [ -n "$pods" ]; then
            for pod in $pods; do
                print_info "Collecting logs from pod: $pod"
                sudo kubectl logs -n "$NAMESPACE" "$pod" > "$OUTPUT_DIR/logs/controller-${pod}.log" 2>&1 || print_warn "Failed to get logs from $pod"
                sudo kubectl logs -n "$NAMESPACE" "$pod" --previous > "$OUTPUT_DIR/logs/controller-${pod}-previous.log" 2>&1 || true
            done
        else
            print_warn "No controller pods found in namespace: $NAMESPACE"
        fi
    else
        print_warn "kubectl not found, skipping controller logs"
    fi
}

collect_migration_logs() {
    print_info "Collecting migration logs from $LOG_DIR"
    if [ ! -d "$LOG_DIR" ]; then
        print_warn "Log directory not found: $LOG_DIR"
        return
    fi
    
    # Convert date filter if provided
    local find_date_filter=""
    if [ -n "$DATE_FILTER" ]; then
        # Check if it's a Unix timestamp or date string
        if [[ "$DATE_FILTER" =~ ^[0-9]+$ ]]; then
            # Unix timestamp
            find_date_filter="-newermt @$DATE_FILTER"
        else
            # Date string (YYYY-MM-DD)
            find_date_filter="-newermt $DATE_FILTER"
        fi
    fi
    
    # Collect logs based on filters
    if [ -n "$VM_FILTER" ]; then
        # Split VM names by comma
        IFS=',' read -ra VMS <<< "$VM_FILTER"
        for vm in "${VMS[@]}"; do
            vm=$(echo "$vm" | xargs) # Trim whitespace
            print_info "Collecting logs for VM: $vm"
            
            # Collect the summary log file (e.g., migration-gk-vm-10-35073.log)
            if [ -f "$LOG_DIR/migration-${vm}.log" ]; then
                cp "$LOG_DIR/migration-${vm}.log" "$OUTPUT_DIR/logs/" 2>/dev/null || true
            fi
            
            # Also try pattern matching for the log file
            find "$LOG_DIR" -maxdepth 1 -type f -name "*${vm}*.log" $find_date_filter -exec cp {} "$OUTPUT_DIR/logs/" \; 2>/dev/null || true
            
            # Collect the directory with timestamped logs (e.g., migration-gk-vm-10-35073/)
            if [ -d "$LOG_DIR/migration-${vm}" ]; then
                print_info "  - Copying directory: migration-${vm}/"
                cp -r "$LOG_DIR/migration-${vm}" "$OUTPUT_DIR/logs/" 2>/dev/null || true
            fi
            
            # Also try pattern matching for the directory
            find "$LOG_DIR" -maxdepth 1 -type d -name "*${vm}*" $find_date_filter -exec cp -r {} "$OUTPUT_DIR/logs/" \; 2>/dev/null || true
            
            # Count collected files
            local pods=$(sudo kubectl get pods -n "$NAMESPACE" --no-headers | grep "^v2v-helper-" | awk '{print $1}')
	    print_info "  - Collected $log_count log file(s) for $vm"
        done
    else
        # Collect all logs
        print_info "Collecting all migration logs..."
        
        # Copy all top-level log files
        find "$LOG_DIR" -maxdepth 1 -type f -name "migration-*.log" $find_date_filter -exec cp {} "$OUTPUT_DIR/logs/" \; 2>/dev/null || true
        
        # Copy all migration directories with their timestamped logs
        for dir in "$LOG_DIR"/migration-*/; do
            if [ -d "$dir" ]; then
                local dirname=$(basename "$dir")
                print_info "  - Copying directory: $dirname/"
                cp -r "$dir" "$OUTPUT_DIR/logs/" 2>/dev/null || true
            fi
        done
        
        # Count total collected
        local file_count=$(find "$OUTPUT_DIR/logs" -type f -name "*.log" 2>/dev/null | wc -l)
        local dir_count=$(find "$OUTPUT_DIR/logs" -mindepth 1 -type d 2>/dev/null | wc -l)
        print_info "  - Collected $file_count log file(s) in $dir_count directory/directories"
    fi
    
    # If no logs were collected, warn the user
    if [ ! "$(ls -A $OUTPUT_DIR/logs 2>/dev/null)" ]; then
        print_warn "No migration logs found matching the criteria"
    fi
}

# Function to collect version information
collect_version_info() {
    print_info "Collecting version information..."
    
    {
        echo "=== vJailbreak Version ==="
        if command -v  kubectl &> /dev/null; then
            sudo kubectl get deployment -n "$NAMESPACE" -o yaml 2>/dev/null | grep -A 5 "image:" || echo "Could not retrieve vJB version"
        fi
        echo ""
        
        echo "=== VDDK Version ==="
        if [ -d "/opt/vmware-vix-disklib-distrib" ]; then
            find /opt/vmware-vix-disklib-distrib -name "*.so" -o -name "*.txt" | head -5
        else
            echo "VDDK not found in /opt/vmware-vix-disklib-distrib"
        fi
        echo ""
        
        echo "=== NBDKit Version ==="
        nbdkit --version 2>/dev/null || echo "nbdkit not found"
        echo ""
        
        echo "=== libnbd Version ==="
        if command -v rpm &> /dev/null; then
            rpm -qa | grep libnbd || echo "libnbd rpm not found"
        elif command -v dpkg &> /dev/null; then
            dpkg -l | grep libnbd || echo "libnbd package not found"
        fi
        echo ""
        
        echo "=== guestfs Version ==="
        if command -v guestfish &> /dev/null; then
            guestfish --version
        else
            echo "guestfish not found"
        fi
        echo ""
        
    } > "$OUTPUT_DIR/versions/version-info.txt"
}

# Function to collect v2v-helper pod information
collect_v2v_helper_info() {
    print_info "Collecting v2v-helper pod information..."
    
    if command -v kubectl &> /dev/null; then
        # Get all v2v-helper pods (including completed ones)
        local pods=$(sudo kubectl get pods -n "$NAMESPACE" --no-headers | grep "^v2v-helper-" | awk '{print $1}')
        
        if [ -n "$pods" ]; then
            for pod in $pods; do
                print_info "Collecting logs from v2v-helper pod: $pod"
                
                # Get pod logs (works even if completed)
                sudo kubectl logs -n "$NAMESPACE" "$pod" > "$OUTPUT_DIR/logs/v2v-helper-${pod}.log" 2>&1 || print_warn "Failed to get logs from $pod"
                
                # Get pod description (shows image, status, etc.)
                sudo kubectl describe pod -n "$NAMESPACE" "$pod" > "$OUTPUT_DIR/system-info/v2v-helper-${pod}-describe.txt" 2>&1 || true
            done
            
            local count=$(echo "$pods" | wc -w)
            print_info "Collected logs from $count v2v-helper pod(s)"
        else
            print_warn "No v2v-helper pods found"
        fi
    fi
}
# Function to collect ConfigMaps
collect_configmaps() {
    print_info "Collecting vJB ConfigMaps..."
    
    if command -v kubectl &> /dev/null; then
        sudo kubectl get configmap -n "$NAMESPACE" -o yaml > "$OUTPUT_DIR/configs/configmaps.yaml" 2>&1 || print_warn "Failed to get ConfigMaps"
    fi
}

# Function to collect system information
collect_system_info() {
    print_info "Collecting system information..."
    
    {
        echo "=== System Information ==="
        echo "Hostname: $(hostname)"
        echo "Date: $(date)"
        echo "Uptime: $(uptime)"
        echo ""
        
        echo "=== Disk Usage ==="
        df -h
        echo ""
        
        echo "=== Memory Usage ==="
        free -h
        echo ""
        
        echo "=== Kubernetes Cluster Info ==="
        if command -v kubectl &> /dev/null; then
            sudo kubectl cluster-info 2>/dev/null || echo "Could not retrieve cluster info"
            echo ""
            sudo kubectl get nodes -o wide 2>/dev/null || echo "Could not retrieve node info"
        fi
        echo ""
        
        echo "=== vJB Namespace Resources ==="
        if command -v  kubectl &> /dev/null; then
            sudo kubectl get all -n "$NAMESPACE" 2>/dev/null || echo "Could not retrieve namespace resources"
        fi
        echo ""
        
    } > "$OUTPUT_DIR/system-info/system-info.txt"
}

# Function to collect debug logs if requested
collect_debug_logs() {
    if [ "$INCLUDE_DEBUG" = true ]; then
        print_info "Collecting debug logs..."
        
        # Collect all pod logs with debug level
        if command -v kubectl &> /dev/null; then
            local pods=$(sudo kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
            
            for pod in $pods; do
                sudo kubectl logs -n "$NAMESPACE" "$pod" --all-containers=true > "$OUTPUT_DIR/logs/debug-${pod}-all-containers.log" 2>&1 || true
            done
        fi
        
        # Collect journalctl logs if available
        if command -v journalctl &> /dev/null; then
            journalctl -u kubelet --since "24 hours ago" > "$OUTPUT_DIR/logs/journalctl-kubelet.log" 2>&1 || true
        fi
    fi
}

# Function to create tarball
create_tarball() {
    local tarball="${OUTPUT_DIR}.tar.gz"
    print_info "Creating support bundle tarball: $tarball"
    
    tar -czf "$tarball" -C "$(dirname $OUTPUT_DIR)" "$(basename $OUTPUT_DIR)" 2>&1
    
    if [ $? -eq 0 ]; then
        print_info "Support bundle created successfully: $tarball"
        print_info "Bundle size: $(du -h $tarball | cut -f1)"
        
        # Clean up temporary directory
        rm -rf "$OUTPUT_DIR"
        
        echo ""
        echo "================================================================"
        echo "Support bundle location: $tarball"
        echo "================================================================"
    else
        print_error "Failed to create tarball"
        exit 1
    fi
}

# Main execution
main() {
    print_info "Starting vJailbreak log collection..."
    echo ""
    
    # Display configuration
    print_info "Configuration:"
    echo "  Output directory: $OUTPUT_DIR"
    echo "  Namespace: $NAMESPACE"
    [ -n "$VM_FILTER" ] && echo "  VM filter: $VM_FILTER"
    [ -n "$DATE_FILTER" ] && echo "  Date filter: $DATE_FILTER"
    [ "$INCLUDE_DEBUG" = true ] && echo "  Debug logs: enabled"
    echo ""
    
    # Collect all information
    collect_migration_logs
    collect_controller_logs
    collect_version_info
    collect_v2v_helper_info
    collect_configmaps
    collect_system_info
    collect_debug_logs
    
    # Create tarball
    create_tarball
    
    print_info "Log collection completed successfully!"
}

# Run main function
main
