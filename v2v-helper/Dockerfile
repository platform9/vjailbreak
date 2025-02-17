# Build the manager binary
FROM fedora:40 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG RELEASE_VERSION

ENV RELEASE_VERSION=$RELEASE_VERSION
# Install dependencies
RUN dnf install -y golang libnbd-devel

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download


# Copy the go source
COPY main.go main.go
COPY migrate/ migrate/
COPY nbd/ nbd/
COPY openstack/ openstack/
COPY reporter/ reporter/
COPY vcenter/ vcenter/
COPY vm/ vm/
COPY virtv2v/ virtv2v/
COPY pkg/ pkg/
COPY Makefile Makefile

# Test
RUN make test

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN make build

FROM fedora:40
ADD https://fedorapeople.org/groups/virt/virtio-win/virtio-win.repo /etc/yum.repos.d/virtio-win.repo
RUN dnf install -y nbdkit nbdkit-vddk-plugin libnbd virt-v2v virtio-win && dnf clean all && rm -rf /var/cache/dnf
COPY --from=builder /workspace/manager /home/fedora/manager
