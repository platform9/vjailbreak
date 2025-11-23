# Stage 1: Build environment
# Using Fedora 42 which includes Go 1.24 required by our go.mod
FROM fedora:42 AS builder

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
COPY manager manager

# Stage 2: Runtime environment
# Using same Fedora version for consistency and required runtime dependencies
FROM fedora:42

WORKDIR /tmp/rpms
COPY nbdkit/*.rpm /tmp/rpms/

RUN dnf install -y \
    ./libnbd-1.22.2-1.fc42.x86_64.rpm \
    ./nbdkit-server-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-basic-plugins-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-basic-filters-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-vddk-plugin-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-ssh-plugin-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-curl-plugin-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-python-plugin-1.42.4-1.fc42.x86_64.rpm \
    ./nbdkit-nbd-plugin-1.42.4-1.fc42.x86_64.rpm \
    ./virt-v2v-2.7.13-1.fc42.x86_64.rpm \
    ./nbdkit-selinux-1.42.4-1.fc42.noarch.rpm && \
    dnf clean all && \
    rm -rf /var/cache/dnf /tmp/rpms/

# Copy patch
COPY disable-fstrim.patch /tmp/disable-fstrim.patch

# Apply patch to installed virt-v2v source files
RUN cd /usr/share/virt-v2v && \
    patch -p1 < /tmp/disable-fstrim.patch

#version lock 
RUN dnf versionlock add nbdkit-1.42.4-1.fc42 nbdkit-vddk-plugin-1.42.4-1.fc42 \
    libnbd-1.22.2-1.fc42 nbdkit-server-1.42.4-1.fc42 nbdkit-basic-plugins-1.42.4-1.fc42 \ 
    nbdkit-basic-filters-1.42.4-1.fc42 nbdkit-curl-plugin-1.42.4-1.fc42 \
    nbdkit-ssh-plugin-1.42.4-1.fc42 nbdkit-python-plugin-1.42.4-1.fc42 \
    nbdkit-nbd-plugin-1.42.4-1.fc42 virt-v2v-2.7.13-1.fc42.x86_64 nbdkit-selinux-1.42.4-1.fc42.noarch

# Copy the binary from builder stage to the runtime environment
COPY --from=builder /workspace/manager /home/fedora/manager

