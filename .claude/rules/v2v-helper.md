---
paths:
  - "v2v-helper/**/*.go"
---

# V2V Helper Development Rules

Rules for developing the v2v-helper migration worker pod that handles disk copy and conversion.

## External Documentation

**ALWAYS consult these resources when working on v2v-helper code:**
- **virt-v2v**: https://libguestfs.org/virt-v2v.1.html - VM conversion tool
- **virt-v2v support**: https://libguestfs.org/virt-v2v-support.1.html - Supported guest OS types and versions
- **libguestfs**: https://libguestfs.org/ - Guest filesystem access and manipulation
- **nbdkit**: https://libguestfs.org/nbdkit.1.html - NBD server for disk operations
- **govmomi**: https://github.com/vmware/govmomi - VMware vSphere API operations

## Testing Requirements

### CGO and Platform Requirements
- v2v-helper tests REQUIRE `CGO_ENABLED=1 GOOS=linux GOARCH=amd64`
- Tests will NOT compile on macOS without Linux cross-compilation toolchain
- Use Docker or Linux VM for development on macOS

### Running Tests
```bash
# From repo root
make test-v2v-helper

# Or directly
cd v2v-helper
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./... -v
```

## Guest OS Support

### Before Adding Support for New OS
- Check virt-v2v support page: https://libguestfs.org/virt-v2v-support.1.html
- Verify virtio driver availability for the OS
- Test initramfs rebuild process for the specific OS version
- Document any OS-specific quirks or workarounds

### Windows Guest Handling
- Ensure virtio-win drivers are properly injected
- Reference: https://github.com/virtio-win/kvm-guest-drivers-windows
- Test driver installation for specific Windows versions

## libguestfs Integration

### Guestfish Operations
- Always check if running in appliance environment (mountpoints under `/sysroot`)
- Use `guestfish command` for running commands in guest chroot
- Handle both appliance and direct execution contexts
- Verify filesystem paths are correct for the execution context

### Error Handling
- libguestfs errors can be cryptic - add detailed logging
- Check guest OS compatibility before attempting operations
- Verify disk format and filesystem types
- Handle read-only vs read-write mount scenarios

## NBD Operations

### NBD Server Management
- Use nbdkit for serving disk images over NBD protocol
- Reference nbdkit documentation for plugin options
- Handle NBD connection lifecycle properly
- Implement proper cleanup on errors

### Disk Streaming
- Monitor NBD transfer progress
- Handle network interruptions gracefully
- Verify data integrity after transfer
- Log transfer metrics for debugging

## VM Conversion Workflow

### Disk Conversion Process
1. Copy disk from VMware (VDDK or NBD)
2. Convert VMDK to QCOW2 using virt-v2v
3. Run virt-v2v-in-place for guest OS preparation
4. Verify and fix initramfs (Linux guests)
5. Fix fstab entries (if needed)
6. Upload to OpenStack

### Error Recovery
- Implement retry logic for transient failures
- Clean up partial conversions on errors
- Report detailed error context to controller
- Preserve logs for debugging

## Module Management

- This is an independent Go module at `v2v-helper/`
- Run `go mod tidy` from `v2v-helper/` directory
- Cross-module imports use full module path

## Build Process

### Building v2v-helper
```bash
# From repo root
make v2v-helper

# Builds both binary and Docker image
```

### Docker Image
- Image includes libguestfs, virt-v2v, nbdkit, and all dependencies
- Base image must support CGO compilation
- VDDK libraries must be available at runtime

## Debugging

### Common Issues
- **Conversion failures**: Check virt-v2v logs, verify guest OS support
- **Initramfs missing virtio**: Check VerifyAndFixInitramfs execution
- **Boot failures**: Check fstab entries, verify /sysroot prefix handling
- **NBD errors**: Check network connectivity, nbdkit logs

### Logging
- Log all virt-v2v operations with full command output
- Include guest OS details in logs
- Log initramfs verification results
- Capture libguestfs debug output for failures

### Testing Specific Guest OS
- Use test VMs with various OS versions
- Verify virtio module inclusion in initramfs
- Test fstab generation and boot process
- Document any OS-specific workarounds
