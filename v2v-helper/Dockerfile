FROM quay.io/platform9/vjailbreak-v2v-helper:v0.1.12 AS builder
WORKDIR /workspace
COPY manager manager

FROM quay.io/platform9/vjailbreak-v2v-helper:v0.1.12
ADD https://fedorapeople.org/groups/virt/virtio-win/virtio-win.repo /etc/yum.repos.d/virtio-win.repo
COPY --from=builder /workspace/manager /home/fedora/manager
RUN chmod +x /home/fedora/manager
