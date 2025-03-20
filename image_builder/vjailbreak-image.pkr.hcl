packer {
  required_plugins {
    qemu = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

source "qemu" "vjailbreak-image" {
  disk_image           = true
  skip_compaction      = true
  iso_url              = "vjailbreak-image.qcow2"
  iso_checksum         = "sha256:4691136dbabceb37d9d03ff10cf717f37cd8076b248c6d74d4ef316f4d4b6f50"
  iso_target_extension = "qcow2"
  output_directory     = "vjailbreak_qcow2"
  vm_name              = "vjailbreak-image.qcow2"
  disk_size            = "10G"
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
  sources = ["source.qemu.vjailbreak-image"]

  provisioner "file" {
    source      = "${path.root}/deploy"
    destination = "etc/pf9/yamls"
  }

  provisioner "file" {
    source      = "${path.root}/scripts/install.sh"
    destination = "/etc/pf9/install.sh"
  }

  provisioner "file" {
    source      = "${path.root}/configs/k3s.env"
    destination = "/etc/pf9/k3s.env"
  }

  provisioner "file" {
    source      = "${path.root}/configs/daemonset.yaml"
    destination = "/etc/pf9/yamls/daemonset.yaml"
  }

  provisioner "shell" {
inline = [
  "sudo curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3",
  "sudo chmod 700 get_helm.sh",
  "sudo ./get_helm.sh",
  "sudo chmod +x /etc/pf9/install.sh",
  "sudo chown root:root /etc/pf9/k3s.env",
  "sudo chmod 644 /etc/pf9/k3s.env",
  "sudo chmod +x /etc/pf9/agent-key-registration.sh",
  "sudo chmod +x /etc/pf9/key-registration-service.sh",
  "sudo chmod 644 /etc/systemd/system/vddk-sync.service",
  "sudo chmod 644 /etc/systemd/system/vddk-sync.timer",
  "sudo chmod +x /etc/pf9/sync-agents.sh",
  "echo '@reboot root /etc/pf9/install.sh' | sudo tee -a /etc/crontab",
  "echo '@reboot root /etc/pf9/agent-key-registration.sh' | sudo tee -a /etc/crontab",
  "echo '@reboot root /etc/pf9/key-registration-service.sh' | sudo tee -a /etc/crontab",
  "sudo systemctl daemon-reload",
  "sudo systemctl enable vddk-sync.timer",
  "sudo systemctl start vddk-sync.timer",
]
  }
}