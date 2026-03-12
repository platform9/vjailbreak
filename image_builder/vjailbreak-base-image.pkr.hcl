packer {
  required_plugins {
    qemu = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

variable "ubuntu_cloud_url" {
  type    = string
  default = "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img"
}

variable "ubuntu_cloud_checksum" {
  type    = string
  default = "file:https://cloud-images.ubuntu.com/jammy/current/SHA256SUMS"
}

variable "k3s_version" {
  type    = string
  default = "v1.31.4+k3s1"
}

variable "helm_version" {
  type    = string
  default = "v3.16.3"
}

source "qemu" "vjailbreak-base-image" {
  disk_image           = true
  skip_compaction      = true
  iso_url              = var.ubuntu_cloud_url
  iso_checksum         = var.ubuntu_cloud_checksum
  iso_target_extension = "qcow2"
  output_directory     = "vjailbreak_base_qcow2"
  vm_name              = "vjailbreak-base-image.qcow2"
  disk_size            = "4G"
  format               = "qcow2"
  headless             = true
  accelerator          = "kvm"
  ssh_username         = "ubuntu"
  ssh_password         = "password"
  ssh_timeout          = "20m"
  cpus                 = 2
  memory               = 2048
  efi_boot             = false
  shutdown_command     = "echo 'password' | sudo -S shutdown -P now"
  boot_wait            = "10s"

  http_directory = "${path.root}/cloudinit/"

  qemuargs = [
    ["-smbios", "type=1,serial=ds=nocloud-net;s=http://{{ .HTTPIP }}:{{ .HTTPPort }}/"]
  ]
}

build {
  sources = ["source.qemu.vjailbreak-base-image"]

  # Copy K3s setup script
  provisioner "file" {
    source      = "${path.root}/scripts/setup-k3s.sh"
    destination = "/tmp/setup-k3s.sh"
  }

  # Copy base images (static container images that don't change per release)
  provisioner "file" {
    source      = "${path.root}/base-images"
    destination = "/home/ubuntu"
  }

  provisioner "shell" {
    environment_vars = [
      "K3S_VERSION=${var.k3s_version}"
    ]
    inline = [
      # Setup K3s binaries and airgap images
      "chmod +x /tmp/setup-k3s.sh",
      "sudo -E /tmp/setup-k3s.sh",

      # Create pf9 directories
      "sudo mkdir -p /etc/pf9",
      "sudo mkdir -p /etc/pf9/images",
      "sudo mkdir -p /etc/pf9/yamls",

      # Move base images to /etc/pf9/images
      "sudo mv /home/ubuntu/base-images/* /etc/pf9/images/",
      "rmdir /home/ubuntu/base-images",

      # Setup virtio-win directory
      "sudo mkdir -p /home/ubuntu/virtio-win",
      "sudo chown -R ubuntu:ubuntu /home/ubuntu/virtio-win",
      "sudo mv /etc/pf9/images/virtio-win.iso /home/ubuntu/virtio-win/virtio-win.iso",
      "sudo mv /etc/pf9/images/virtio-win-server12.iso /home/ubuntu/virtio-win/virtio-win-server12.iso",

      # Update and install required packages
      "sudo apt-get update",
      "sudo DEBIAN_FRONTEND=noninteractive apt-get dist-upgrade -y",
      "sudo apt-get install -y --no-install-recommends cron curl ca-certificates python3-openstackclient netcat-openbsd vim telnet dnsutils net-tools iputils-ping traceroute tcpdump iproute2 bind9-dnsutils nmap htop iotop strace lsof",
      "sudo systemctl enable cron",

      # Install Helm
      "curl -fsSL -o /tmp/get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3",
      "chmod 700 /tmp/get_helm.sh",
      "DESIRED_VERSION=${var.helm_version} /tmp/get_helm.sh",
      "rm /tmp/get_helm.sh",

      # Pull ingress-nginx helm chart
      "sudo helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx",
      "sudo helm repo update",
      "sudo helm pull ingress-nginx/ingress-nginx --untar --untardir /etc/pf9/",

      # Cleanup
      "sudo apt-get clean",
      "sudo rm -rf /var/lib/apt/lists/*",
    ]
  }
}
