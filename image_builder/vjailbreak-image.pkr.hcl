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
  iso_checksum         = "sha256:67d312441a401c75389b063e1331c5fa11f35813f531dcbcceba765a4895251f"
  iso_target_extension = "qcow2"
  output_directory     = "vjailbreak_qcow2"
  vm_name              = "vjailbreak-image.qcow2"
  disk_size            = "15G"
  format               = "qcow2"
  headless             = true
  accelerator          = "kvm"
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

    provisioner "shell" {
    inline = [
      "cat <<EOF | sudo tee /etc/systemd/system/setup-k3s.service",
      "[Unit]",
      "Description=Setup and Start K3s",
      "After=network-online.target",
      "",
      "[Service]",
      "Type=oneshot",
      "ExecStart=/usr/local/bin/k3s server &",
      "RemainAfterExit=true",
      "",
      "[Install]",
      "WantedBy=multi-user.target",
      "EOF",
      "sudo systemctl enable setup-k3s"
    ]
  }
  
  provisioner "shell" {
    inline = [
      "while ! systemctl is-active --quiet k3s; do sleep 10; done",
    ]
  }

  provisioner "shell" {
    inline = [
      "sudo kubectl --request-timeout=300s apply --server-side -f /tmp/yamls/kube-prometheus/manifests/setup",
      "sudo kubectl wait --for condition=Established --all CustomResourceDefinition --namespace=monitoring",
      "sudo kubectl --request-timeout=300s apply -f /tmp/yamls/kube-prometheus/manifests/",
      "sudo kubectl --request-timeout=300s apply -f /tmp/yamls/"
    ]
  }

  provisioner "shell" {
    inline = [
      "sudo k3s crictl rmi --prune",
      "sudo rm -rf /home/ubuntu/vmware-vix-disklib-distrib"
    ]
  }
}
