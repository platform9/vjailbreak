package cli

import (
	"fmt"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/targets/vcenter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var Creds = vcenter.VMCenterAccessInfo{}

func populateCredsFromCMD(cmd *cobra.Command) {
	if val, err := cmd.Flags().GetString("host"); err == nil {
		Creds.HostnameOrIP = val
	}
	if val, err := cmd.Flags().GetString("port"); err == nil {
		Creds.Port = val
	}
	if val, err := cmd.Flags().GetString("datacenter"); err == nil {
		Creds.Datacenter = val
	}
	if val, err := cmd.Flags().GetString("username"); err == nil {
		Creds.Username = val
	}
	if val, err := cmd.Flags().GetString("password"); err == nil {
		Creds.Password = val
	}
}

var vcenterCmd = &cobra.Command{
	Use:   "vcenter",
	Short: "target vcenter",
	Long:  "target vcenter to fetch vcenter details and manage vcenter resources",
	Run: func(cmd *cobra.Command, args []string) {
		populateCredsFromCMD(cmd)
		fmt.Println(cmd.UsageString())
	},
}

var listVMsCmd = &cobra.Command{
	Use:   "list",
	Short: "list vcenter resources",
	Long:  "list vcenter resources",
	Run: func(cmd *cobra.Command, args []string) {
		populateCredsFromCMD(cmd)
		vcenter := vcenter.Vcenter{}
		res, err := vcenter.ListVMs(Creds)
		for k, v := range res {
			fmt.Println(k, v)
		}
		if err != nil {
			logrus.Error(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(vcenterCmd)
	vcenterCmd.PersistentFlags().StringP("host", "H", "", "Set the IP-Address or hostname where vcenter is running")
	vcenterCmd.PersistentFlags().StringP("port", "p", "443", "Set the sdk port to open")
	vcenterCmd.PersistentFlags().StringP("datacenter", "d", "", "Set the datacenter to target")
	vcenterCmd.PersistentFlags().StringP("username", "U", "", "Set the username to use")
	vcenterCmd.PersistentFlags().StringP("password", "P", "", "Set the password to use")
	vcenterCmd.AddCommand(listVMsCmd)
}
