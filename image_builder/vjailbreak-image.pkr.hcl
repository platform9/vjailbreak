packer {
  required_plugins {
    qemu = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/qemu"
    }
  }
}

variable "base_image_url" {
  type        = string
  description = "URL or local path to the vjailbreak base image. Build with vjailbreak-base-image.pkr.hcl first."
  default     = "vjailbreak_base_qcow2/vjailbreak-base-image.qcow2"
}

variable "base_image_checksum" {
  type        = string
  description = "Checksum of the base image. Set to 'none' to skip checksum verification for local files."
  default     = "none"
}

source "qemu" "vjailbreak-image" {
  disk_image           = true
  skip_compaction      = true
  iso_url              = var.base_image_url
  iso_checksum         = var.base_image_checksum
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
    destination = "/tmp/yamls"
  }

  provisioner "file" {
    source      = "${path.root}/scripts/install.sh"
    destination = "/tmp/install.sh"
  }
  provisioner "file" {
    source      = "${path.root}/scripts/pf9-htpasswd.sh"
    destination = "/tmp/pf9-htpasswd.sh"
  }

  provisioner "file" {
    source      = "${path.root}/scripts/log_collector.sh"
    destination = "/tmp/log_collector.sh"
  }
  provisioner "file" {
    source      = "${path.root}/scripts/user_setup_daemon.sh"
    destination = "/tmp/user_setup_daemon.sh"
  }
  provisioner "file" {
    source      = "${path.root}/configs/k3s.env"
    destination = "/tmp/k3s.env"
  }

  provisioner "file" {
    source      = "${path.root}/configs/daemonset.yaml"
    destination = "/tmp/daemonset.yaml"
  }

  provisioner "file" {
    source      = "${path.root}/configs/rsyncd.conf"
    destination = "/tmp/rsyncd.conf"
  }
  
  provisioner "file" {
    source      = "${path.root}/configs/env"
    destination = "/tmp/env"
  }

  provisioner "file" {
    source      = "${path.root}/configs/vjailbreak-settings.yaml"
    destination = "/tmp/vjailbreak-settings.yaml"
  }

  provisioner "file" {
    source      = "${path.root}/cronjob/version-checker.yaml"
    destination = "/tmp/version-checker.yaml"
  }

  provisioner "file" {
    source      = "${path.root}/images"
    destination = "/home/ubuntu"
  }

  provisioner "file" {
    source      = "${path.root}/opensource.txt"
    destination = "/tmp/opensource.txt"
  }
  provisioner "shell" {
    inline = [
    # Copy version-specific images to /etc/pf9/images (base images already there from base image)
    "sudo cp /home/ubuntu/images/* /etc/pf9/images/",
    "rm -rf /home/ubuntu/images",

    # Move scripts and configs
    "sudo mv /tmp/install.sh /etc/pf9/install.sh",
    "sudo mv /tmp/pf9-htpasswd.sh /etc/pf9/pf9-htpasswd.sh",
    "sudo mv /tmp/log_collector.sh /etc/pf9/log_collector.sh",
    "sudo mv /tmp/k3s.env /etc/pf9/k3s.env",

    # Move yamls (merge with existing /etc/pf9/yamls from base image)
    "sudo cp -r /tmp/yamls/* /etc/pf9/yamls/",
    "rm -rf /tmp/yamls",
    "sudo mv /tmp/rsyncd.conf /etc/pf9/rsyncd.conf",
    "sudo mv /tmp/daemonset.yaml /etc/pf9/yamls/daemonset.yaml",
    "sudo mv /tmp/env /etc/pf9/env",
    "sudo mv /tmp/vjailbreak-settings.yaml /etc/pf9/yamls/vjailbreak-settings.yaml",
    "sudo mv /tmp/version-checker.yaml /etc/pf9/yamls/version-checker.yaml",
    "sudo mv /tmp/opensource.txt /home/ubuntu/opensource.txt",

    # Set permissions
    "sudo chmod +x /etc/pf9/install.sh",
    "sudo chmod +r /etc/pf9/pf9-htpasswd.sh",
    "sudo chmod +x /etc/pf9/log_collector.sh",
    "sudo chown root:root /etc/pf9/k3s.env",
    "sudo chmod 644 /etc/pf9/k3s.env",
    "sudo chmod 644 /etc/pf9/env",

    # Run user setup daemon
    "sudo chmod +x /tmp/user_setup_daemon.sh",
    "sudo df -h",
    "sudo bash /tmp/user_setup_daemon.sh",

    # Setup cron job for install.sh on reboot
    "echo '@reboot root /etc/pf9/install.sh' | sudo tee -a /etc/crontab",
    ]
  }
}

