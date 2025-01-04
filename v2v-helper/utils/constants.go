package utils

const (
	RhelFirstBootScript = `#!/bin/bash
nmcli -t -f NAME connection show | while read -r conn; do
    nmcli con modify "$conn" ipv4.method auto ipv4.address "" ipv4.gateway ""
    nmcli con modify "$conn" ipv6.method auto ipv6.address "" ipv6.gateway ""
    nmcli con reload
    nmcli con down "$conn"
    nmcli con up "$conn"
done
systemctl enable --now serial-getty@ttyS0.service`
)
