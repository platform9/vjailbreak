# Stage 1: Build environment
# Using Fedora 42 which includes Go 1.24 required by our go.mod
FROM quay.io/platform9/vjailbreak-v2v-helper:v0.1.12 AS builder

# Build arguments for multi-platform support and versioning
ARG TARGETOS
ARG TARGETARCH
ARG RELEASE_VERSION
ENV RELEASE_VERSION=$RELEASE_VERSION


# Set up build workspace
WORKDIR /workspace

# Copy pre-built binary from local build
# Note: The binary is built by 'make -C v2v-helper build' before docker build
# This is part of the VM migration toolchain for converting between different platforms
COPY manager manager

# Stage 2: Runtime environment
# Using same Fedora version for consistency and required runtime dependencies
FROM quay.io/platform9/vjailbreak-v2v-helper:v0.1.12

# Add virtio-win repository for Windows drivers
# Required for VM migration and conversion support
# This provides Windows VirtIO drivers needed during VM conversion
ADD https://fedorapeople.org/groups/virt/virtio-win/virtio-win.repo /etc/yum.repos.d/virtio-win.repo

# Copy the binary from builder stage to the runtime environment
COPY --from=builder /workspace/manager /home/fedora/manager