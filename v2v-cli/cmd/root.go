package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "v2v-helper-cli",
	Short: "A CLI tool to migrate VMs from VMware to Openstack.",
	Long:  `A CLI tool to migrate VMs from VMware to Openstack.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Register your commands here
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(statusCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	migrateCmd.PersistentFlags().StringP("admin-file", "a", "admin.rc", "Path to the admin file")
	migrateCmd.PersistentFlags().StringP("vcenter-user", "u", "", "Username for vCenter")
	migrateCmd.PersistentFlags().StringP("vcenter-password", "p", "", "Password for vCenter")
	migrateCmd.PersistentFlags().StringP("vcenter-host", "", "", "vCenter host")
	migrateCmd.PersistentFlags().StringP("source-vm-name", "s", "", "Name of the source VM")
	migrateCmd.PersistentFlags().StringP("convert", "c", "true", "Convert the VM to Openstack format")
	migrateCmd.PersistentFlags().StringP("vcenter-insecure", "i", "true", "Ignore SSL certificate verification")
	migrateCmd.PersistentFlags().StringP("neutron-network-name", "n", "vlan-218-uservm-network-1", "Neutron network name")
	migrateCmd.PersistentFlags().StringP("os-type", "o", "Linux", "OS type of the VM")
	migrateCmd.PersistentFlags().StringP("virtio-win-iso", "w", "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso", "URL to the virtio-win iso")

}
