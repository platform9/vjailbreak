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
  iso_url              = "https://ngpc-prod-public-data.s3.us-east-2.amazonaws.com/vjailbreak/vjailbreak-image.qcow2"
  iso_checksum         = "sha256:8da72a4179373fcb442c0b274f4b3c3d8d0d5b210d6cd44e296edb2f696c36e8"
  iso_target_extension = "qcow2"
  output_directory     = "vjailbreak_qcow2"
  disk_size            = "15G"
  format               = "qcow2"
  headless             = true
  accelerator          = "none" # "kvm" if not running in docker
  ssh_password         = "password"
  ssh_username         = "ubuntu"
  ssh_timeout          = "20m"
  cpus                 = 2
  memory               = 2048
  efi_boot             = false
  shutdown_command     = "echo 'password' | sudo -S shutdown -P now"

  boot_wait = "10s"

  # Location of Cloud-Init / Autoinstall Configuration files
  # Will be served via an HTTP Server from Packer
  http_directory = "cloudinit/"


  qemuargs = [
    ["-smbios", "type=1,serial=ds=nocloud-net;s=http://{{ .HTTPIP }}:{{ .HTTPPort }}/"]
  ]
}

build {
  sources = ["source.qemu.vjailbreak-image"]

  provisioner "file" {
    source      = "deploy"
    destination = "/tmp/yamls"
  }

  provisioner "shell" {
    inline = [
      "while ! systemctl is-active --quiet k3s; do sleep 10; done",
      "sudo kubectl --request-timeout=300s apply -f /tmp/yamls/"
    ]
  }
}
