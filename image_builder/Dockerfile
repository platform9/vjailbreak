# First stage: Build with Packer
FROM hashicorp/packer:light AS build-stage
RUN apk add --no-cache qemu-system-x86_64 qemu-img
WORKDIR /workspace
COPY cloudinit/ cloudinit/
COPY deploy/ deploy/
COPY vjailbreak-image.pkr.hcl vjailbreak-image.pkr.hcl

RUN packer init .
RUN packer fmt .
RUN packer validate .
RUN PACKER_LOG=1 packer build .

# Second stage: Copy the desired folder for extraction
FROM scratch AS export-stage

# Copy the vjailbreak_qcow2 folder from the build stage
COPY --from=build-stage /workspace/vjailbreak_qcow2 /vjailbreak_qcow2