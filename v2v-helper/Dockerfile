# Stage 1: Build environment
# Using Fedora 44 to match runtime stage and vendored libnbd-devel
FROM fedora:44 AS builder

# Build arguments for multi-platform support and versioning
ARG TARGETOS
ARG TARGETARCH
ARG RELEASE_VERSION
ENV RELEASE_VERSION=$RELEASE_VERSION

# Install build dependencies
# - golang: Required for building the application
# - libnbd-devel: Development files for NBD (Network Block Device) support
#   Installed from vendored fc44 RPM to match the runtime libnbd version exactly
WORKDIR /tmp/rpms
COPY v2v-helper/nbdkit/fc44-rpms/libnbd-devel-1.25.5-1.fc44.x86_64.rpm /tmp/rpms/
RUN dnf install -y golang \
    ./libnbd-devel-1.25.5-1.fc44.x86_64.rpm && \
    dnf clean all && rm -rf /var/cache/dnf /var/cache/yum /tmp/rpms/

# Set up build workspace
WORKDIR /workspace

# Copy pre-built binary from local build
# Note: The binary is built by 'make -C v2v-helper build' before docker build
# This is part of the VM migration toolchain for converting between different platforms
COPY v2v-helper/manager manager

# Stage 2: Runtime environment
# Using Fedora 44 to match vendored RPM set (virt-v2v 2.11.8, nbdkit 1.47.5, libnbd 1.25.5)
FROM fedora:44

WORKDIR /tmp/rpms
COPY v2v-helper/nbdkit/fc44-rpms/*.rpm /tmp/rpms/

# Install runtime dependencies with size optimizations:
# - --nodocs: Skip documentation files
# - --setopt=install_weak_deps=False: Skip weak dependencies
# - Clean up aggressively after install
# All packages installed from vendored fc44 RPMs for reproducible builds.
# netplan is the only package pulled from the repo (no fc44 RPM vendored).
RUN dnf install -y \
    --nodocs \
    --setopt=install_weak_deps=False \
    ./libnbd-1.25.5-1.fc44.x86_64.rpm \
    ./nbdkit-server-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-basic-plugins-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-basic-filters-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-vddk-plugin-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-ssh-plugin-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-curl-plugin-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-python-plugin-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-nbd-plugin-1.47.5-1.fc44.x86_64.rpm \
    ./nbdkit-selinux-1.47.5-1.fc44.noarch.rpm \
    ./virt-v2v-2.11.8-1.pf9.fc43.x86_64.rpm \
    ./libguestfs-1.59.8-1.fc44.x86_64.rpm \
    ./libguestfs-appliance-1.59.8-1.fc44.x86_64.rpm \
    ./guestfs-tools-1.55.8-1.fc44.x86_64.rpm \
    netplan && \
    dnf clean all && \
    rm -rf /var/cache/dnf /var/cache/yum /tmp/rpms/ && \
    rm -rf /usr/share/doc /usr/share/man /usr/share/info /usr/share/locale && \
    find /var/log -type f -delete

# Version lock to prevent accidental upgrades
# Install versionlock plugin without docs to save space
RUN dnf install -y --nodocs --setopt=install_weak_deps=False python3-dnf-plugin-versionlock && \
    dnf versionlock add \
    nbdkit-1.47.5-1.fc44 \
    nbdkit-vddk-plugin-1.47.5-1.fc44 \
    libnbd-1.25.5-1.fc44 \
    nbdkit-server-1.47.5-1.fc44 \
    nbdkit-basic-plugins-1.47.5-1.fc44 \
    nbdkit-basic-filters-1.47.5-1.fc44 \
    nbdkit-curl-plugin-1.47.5-1.fc44 \
    nbdkit-ssh-plugin-1.47.5-1.fc44 \
    nbdkit-python-plugin-1.47.5-1.fc44 \
    nbdkit-nbd-plugin-1.47.5-1.fc44 \
    nbdkit-selinux-1.47.5-1.fc44.noarch \
    virt-v2v-2.11.8-1.pf9.fc43.x86_64 \
    libguestfs-1.59.8-1.fc44.x86_64 \
    libguestfs-appliance-1.59.8-1.fc44.x86_64 \
    guestfs-tools-1.55.8-1.fc44.x86_64 && \
    dnf clean all && \
    rm -rf /var/cache/dnf /var/cache/yum

# Copy the binary from builder stage to the runtime environment
COPY --from=builder /workspace/manager /home/fedora/manager

# Copy scripts directory for migration utilities
COPY scripts/generate-mount-persistence.sh /home/fedora/generate-mount-persistence.sh
COPY scripts/get-bootable-partition.sh /home/fedora/get-bootable-partition.sh
COPY scripts/generate-udev-mapping.sh /home/fedora/generate-udev-mapping.sh
COPY scripts/firstboot/windows/NIC-Recovery /home/fedora/NIC-Recovery
COPY scripts/firstboot/store /home/fedora/store
COPY scripts/offline-vmware-cleanup.sh /home/fedora/offline-vmware-cleanup.sh
COPY scripts/firstboot/linux/vmware-tools-cleanup.sh /home/fedora/vmware-tools-cleanup.sh
COPY scripts/mkinitrd-lvm-wrapper.sh /home/fedora/mkinitrd-lvm-wrapper.sh
COPY scripts/firstboot/windows/Firstboot-Init-Windows.bat /home/fedora/Firstboot-Init-Windows.sh
COPY scripts/99-l2-Netplan.yaml /home/fedora/99-l2-Netplan.yaml
