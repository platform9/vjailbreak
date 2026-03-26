# Stage 1: Build environment
# Using Fedora 42 which includes Go 1.24 required by our go.mod
FROM fedora:43 AS builder

# Build arguments for multi-platform support and versioning
ARG TARGETOS
ARG TARGETARCH
ARG RELEASE_VERSION
ENV RELEASE_VERSION=$RELEASE_VERSION

# Install build dependencies
# - golang: Required for building the application (Go 1.24.1 from Fedora 42)
# - libnbd-devel: Development files for NBD (Network Block Device) support
RUN dnf install -y golang libnbd-devel

# Set up build workspace
WORKDIR /workspace

# Copy pre-built binary from local build
# Note: The binary is built by 'make -C v2v-helper build' before docker build
# This is part of the VM migration toolchain for converting between different platforms
COPY v2v-helper/manager manager

# Stage 2: Runtime environment
# Using same Fedora version for consistency and required runtime dependencies
FROM fedora:43

WORKDIR /tmp/rpms
COPY v2v-helper/nbdkit/*.rpm /tmp/rpms/

# Install runtime dependencies with size optimizations:
# - --nodocs: Skip documentation files
# - --setopt=install_weak_deps=False: Skip weak dependencies
# - Individual installs for better layer caching and debugging
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./libnbd-1.22.2-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-server-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-basic-plugins-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-basic-filters-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-vddk-plugin-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-ssh-plugin-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-curl-plugin-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-python-plugin-1.42.4-1.fc42.x86_64.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-nbd-plugin-1.42.4-1.fc42.x86_64.rpm
RUN dnf install -y --nodocs --setopt=install_weak_deps=False virt-v2v
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False ./nbdkit-selinux-1.42.4-1.fc42.noarch.rpm
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False guestfs-tools
RUN dnf install -y --nodocs --setopt=install_weak_deps=False netplan
RUN dnf clean all
RUN rm -rf /var/cache/dnf /var/cache/yum /tmp/rpms/
RUN rm -rf /usr/share/doc /usr/share/man /usr/share/info /usr/share/locale
RUN find /var/log -type f -delete

# Version lock to prevent accidental upgrades
# Install versionlock plugin without docs to save space
# RUN dnf install -y --nodocs --setopt=install_weak_deps=False python3-dnf-plugin-versionlock
# RUN dnf versionlock add nbdkit-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-vddk-plugin-1.42.4-1.fc42
# RUN dnf versionlock add libnbd-1.22.2-1.fc42
# RUN dnf versionlock add nbdkit-server-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-basic-plugins-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-basic-filters-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-curl-plugin-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-ssh-plugin-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-python-plugin-1.42.4-1.fc42
# RUN dnf versionlock add nbdkit-nbd-plugin-1.42.4-1.fc42
# RUN dnf versionlock add virt-v2v-2.7.17-1.fc42.x86_64
# RUN dnf versionlock add nbdkit-selinux-1.42.4-1.fc42.noarch
# RUN dnf clean all
# RUN rm -rf /var/cache/dnf /var/cache/yum

# Copy the binary from builder stage to the runtime environment
COPY --from=builder /workspace/manager /home/fedora/manager

# Copy scripts directory for migration utilities
COPY scripts/generate-mount-persistence.sh /home/fedora/generate-mount-persistence.sh
COPY scripts/get-bootable-partition.sh /home/fedora/get-bootable-partition.sh
COPY scripts/generate-udev-mapping.sh /home/fedora/generate-udev-mapping.sh
COPY scripts/firstboot/windows/NIC-Recovery /home/fedora/NIC-Recovery
COPY scripts/firstboot/store /home/fedora/store
COPY scripts/firstboot/windows/Firstboot-Init-Windows.bat /home/fedora/Firstboot-Init-Windows.sh
COPY scripts/99-l2-Netplan.yaml /home/fedora/99-l2-Netplan.yaml