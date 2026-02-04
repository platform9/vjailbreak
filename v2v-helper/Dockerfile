# Stage 1: Build environment
# Using Fedora 43 which includes required dependencies for virt-v2v 2.10.0
# (libguestfs 1.58.0, nbdkit 1.46.1, libnbd 1.24.0)
FROM fedora:43 AS builder

# Build arguments for multi-platform support and versioning
ARG TARGETOS
ARG TARGETARCH
ARG RELEASE_VERSION
ENV RELEASE_VERSION=$RELEASE_VERSION

# Install build dependencies with exact versions
# - golang: Required for building the application (Go 1.25.6 from Fedora 43)
# - libnbd-devel: Development files for NBD (Network Block Device) support
RUN dnf install -y \
    'golang-1.25.6-1.fc43' \
    'libnbd-devel-1.24.0-1.fc43'

# Set up build workspace
WORKDIR /workspace

# Copy pre-built binary from local build
# Note: The binary is built by 'make -C v2v-helper build' before docker build
# This is part of the VM migration toolchain for converting between different platforms
COPY v2v-helper/manager manager

# Stage 2: Runtime environment
# Using Fedora 43 for virt-v2v 2.10.0 compatibility
FROM fedora:43

WORKDIR /tmp/rpms
COPY v2v-helper/nbdkit/*.rpm /tmp/rpms/

# Install runtime dependencies with size optimizations:
# - --nodocs: Skip documentation files
# - --setopt=install_weak_deps=False: Skip weak dependencies
# - Clean up aggressively after install
# Exact versions from Fedora 43 (no patch version changes possible):
# - virt-v2v-2.10.0-1.fc43
# - libguestfs-1.58.0-1.fc43
# - nbdkit-1.46.1-1.fc43
# - libnbd-1.24.0-1.fc43
# - guestfs-tools-1.55.3-1.fc43
RUN dnf install -y \
    --nodocs \
    --setopt=install_weak_deps=False \
    ./libnbd-1.24.0-1.fc43.x86_64.rpm \
    ./nbdkit-server-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-basic-plugins-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-basic-filters-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-vddk-plugin-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-ssh-plugin-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-curl-plugin-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-python-plugin-1.46.1-1.fc43.x86_64.rpm \
    ./nbdkit-nbd-plugin-1.46.1-1.fc43.x86_64.rpm \
    ./virt-v2v-2.10.0-1.fc43.x86_64.rpm \
    ./libguestfs-1.58.0-1.fc43.x86_64.rpm \
    ./nbdkit-selinux-1.46.1-1.fc43.noarch.rpm \
    ./guestfs-tools-1.55.3-1.fc43.x86_64.rpm \
    'netplan-1.1.2-4.fc43' && \
    dnf clean all && \
    rm -rf /var/cache/dnf /var/cache/yum /tmp/rpms/ && \
    rm -rf /usr/share/doc /usr/share/man /usr/share/info /usr/share/locale && \
    find /var/log -type f -delete

# Version lock to prevent accidental upgrades
# Install versionlock plugin without docs to save space
# Lock exact versions to prevent any patch version changes
RUN dnf install -y --nodocs --setopt=install_weak_deps=False 'python3-dnf-plugin-versionlock-4.10.1-6.fc43' && \
    dnf versionlock add \
    'nbdkit-1.46.1-1.fc43' \
    'nbdkit-server-1.46.1-1.fc43' \
    'nbdkit-basic-plugins-1.46.1-1.fc43' \
    'nbdkit-basic-filters-1.46.1-1.fc43' \
    'nbdkit-vddk-plugin-1.46.1-1.fc43' \
    'nbdkit-ssh-plugin-1.46.1-1.fc43' \
    'nbdkit-curl-plugin-1.46.1-1.fc43' \
    'nbdkit-python-plugin-1.46.1-1.fc43' \
    'nbdkit-nbd-plugin-1.46.1-1.fc43' \
    'nbdkit-selinux-1.46.1-1.fc43' \
    'libnbd-1.24.0-1.fc43' \
    'virt-v2v-2.10.0-1.fc43' \
    'libguestfs-1.58.0-1.fc43' \
    'guestfs-tools-1.55.3-1.fc43' \
    'netplan-1.1.2-4.fc43' \
    'golang-1.25.6-1.fc43' \
    'libnbd-devel-1.24.0-1.fc43' \
    'python3-dnf-plugin-versionlock-4.10.1-6.fc43' && \
    dnf clean all && \
    rm -rf /var/cache/dnf /var/cache/yum

# Copy the binary from builder stage to the runtime environment
COPY --from=builder /workspace/manager /home/fedora/manager

# Copy scripts directory for migration utilities
COPY scripts/generate-mount-persistence.sh /home/fedora/generate-mount-persistence.sh
COPY scripts/get-bootable-partition.sh /home/fedora/get-bootable-partition.sh
COPY scripts/generate-udev-mapping.sh /home/fedora/generate-udev-mapping.sh
COPY scripts/firstboot/windows/NIC-Recovery /home/fedora/NIC-Recovery