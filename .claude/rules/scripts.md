---
paths:
  - "scripts/**/*.sh"
---

# Script Development Rules

Rules for developing bash scripts used in vJailbreak, particularly those run via guestfish in guest VM environments.

## External Documentation

**Reference libguestfs documentation when working with scripts:**
- **libguestfs**: https://libguestfs.org/ - Understanding guestfish environment
- **guestfish**: https://libguestfs.org/guestfish.1.html - Guest filesystem manipulation

## Critical Context: Guestfish Appliance Environment

### The /sysroot Problem

**CRITICAL**: When scripts run via guestfish, the execution environment is different from normal:
- Guest filesystems are mounted under `/sysroot` prefix in the appliance
- `/proc/mounts` shows paths like `/sysroot`, `/sysroot/boot`, `/sysroot/home`
- Scripts must detect this environment and handle paths correctly

### Environment Detection

Scripts MUST detect if running in guestfish appliance:
```bash
# Check if running in guestfish appliance
if mountpoint -q /sysroot; then
    IN_APPLIANCE=true
    # Guest root is at /sysroot
    GUEST_ROOT="/sysroot"
else
    IN_APPLIANCE=false
    # Running directly in guest
    GUEST_ROOT=""
fi
```

## Known Issues & Solutions

### Fstab /sysroot Prefix Bug

**Problem**: `generate-mount-persistence.sh` was writing appliance mountpoints (with `/sysroot` prefix) directly into guest's `/etc/fstab`, causing boot failures.

**Solution implemented**:
1. Detect guestfish appliance environment
2. Strip `/sysroot` prefix from mountpoints before writing to fstab
3. Write to `/sysroot/etc/fstab` (not `/etc/fstab`) when in appliance
4. Use exact field match for dedup instead of substring grep

**Example**:
```bash
# BAD - writes appliance paths to guest fstab
echo "/sysroot / ext4 defaults 0 1" >> /etc/fstab

# GOOD - strips prefix and writes to correct location
echo "/ / ext4 defaults 0 1" >> /sysroot/etc/fstab
```

### Path Handling Rules

When working with guest filesystem paths:

1. **Reading from guest**: Prefix with `/sysroot` in appliance
   ```bash
   cat /sysroot/etc/fstab  # Read guest's fstab
   ```

2. **Writing to guest**: Prefix with `/sysroot` in appliance
   ```bash
   echo "data" > /sysroot/etc/config  # Write to guest's /etc/config
   ```

3. **Processing paths**: Strip `/sysroot` before writing to guest files
   ```bash
   # If path is /sysroot/boot, write /boot to fstab
   MOUNT_PATH=$(echo "$FULL_PATH" | sed 's|^/sysroot||')
   ```

## Script Standards

### Shebang and Options
```bash
#!/bin/bash
set -euo pipefail  # Exit on error, undefined vars, pipe failures
```

### Error Handling
- Check command exit codes
- Provide meaningful error messages
- Log errors to stderr
- Exit with appropriate error codes

### Logging
```bash
log_info() {
    echo "[INFO] $*" >&2
}

log_error() {
    echo "[ERROR] $*" >&2
}
```

### Input Validation
- Validate all script arguments
- Check for required files/directories
- Verify environment variables
- Fail fast on invalid input

## Specific Script Guidelines

### generate-mount-persistence.sh

**Purpose**: Generate persistent mount entries in guest's `/etc/fstab`

**Critical requirements**:
- Detect guestfish appliance environment
- Strip `/sysroot` prefix from mountpoints
- Write to `/sysroot/etc/fstab` in appliance
- Use exact field matching for deduplication
- Preserve existing fstab entries
- Handle special filesystems (tmpfs, devtmpfs, etc.)

### generate-udev-mapping.sh

**Purpose**: Generate udev rules for network interface naming

**Requirements**:
- Handle both appliance and direct execution
- Generate stable network interface names
- Preserve MAC address mappings

### Firstboot Scripts

Located in `scripts/firstboot/`:
- **linux/**: Scripts run on first boot of Linux VMs
- **windows/**: Scripts run on first boot of Windows VMs
- **store/**: Shared utilities and data

**Requirements**:
- Scripts must be idempotent
- Handle missing dependencies gracefully
- Log all operations
- Clean up temporary files

## Testing Scripts

### Manual Testing
```bash
# Test in guestfish environment
guestfish -a disk.qcow2 -i

# Run script
><fs> command "/path/to/script.sh"

# Verify results
><fs> cat /etc/fstab
```

### Validation
- Test with various guest OS types
- Verify filesystem paths are correct
- Check fstab entries are valid
- Test boot process after script execution

## Common Patterns

### Safe File Editing
```bash
# Create backup before modifying
cp /sysroot/etc/fstab /sysroot/etc/fstab.backup

# Make changes
# ...

# Verify changes
if ! validate_fstab /sysroot/etc/fstab; then
    mv /sysroot/etc/fstab.backup /sysroot/etc/fstab
    log_error "Invalid fstab, restored backup"
    exit 1
fi
```

### Deduplication
```bash
# Use exact field matching, not substring
if ! grep -q "^${DEVICE}[[:space:]]" /sysroot/etc/fstab; then
    echo "${DEVICE} ${MOUNT} ${FSTYPE} ${OPTIONS} 0 0" >> /sysroot/etc/fstab
fi
```

## Debugging

### Common Issues
- **Wrong paths in fstab**: Check for `/sysroot` prefix handling
- **Boot failures**: Verify fstab entries are correct
- **Permission errors**: Check if running with proper privileges
- **Missing files**: Verify paths are correct for execution context

### Debug Logging
```bash
# Enable debug output
set -x  # Print commands as they execute

# Or use conditional debug
if [ "${DEBUG:-}" = "1" ]; then
    set -x
fi
```

### Testing in Isolation
```bash
# Test script logic without modifying guest
DRY_RUN=1 ./script.sh

# In script:
if [ "${DRY_RUN:-}" = "1" ]; then
    echo "Would write: $DATA"
else
    echo "$DATA" > /sysroot/etc/config
fi
```

## Security Considerations

- Validate all input from guest filesystem
- Avoid command injection vulnerabilities
- Use quotes around variables
- Sanitize paths before use
- Don't trust guest-provided data

## Documentation

- Add comments explaining complex logic
- Document expected environment
- List required tools/dependencies
- Include usage examples
- Document known limitations
