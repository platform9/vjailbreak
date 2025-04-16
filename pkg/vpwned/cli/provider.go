package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var creds = providers.BMAccessInfo{}
var currProvider providers.BMCProvider

func populateBMCredsFromCMD(cmd *cobra.Command) {
	if val, err := cmd.Flags().GetString("api_key"); err == nil {
		creds.APIKey = val
	}
	if val, err := cmd.Flags().GetString("base_url"); err == nil {
		creds.BaseURL = val
	}
	if val, err := cmd.Flags().GetBool("use_insecure"); err == nil {
		creds.UseInsecure = val
	}
	if val, err := cmd.Flags().GetString("username"); err == nil {
		creds.Username = val
	}
	if val, err := cmd.Flags().GetString("password"); err == nil {
		creds.Password = val
	}
}

func initProvider(name string) {
	logrus.Info("initializing provider: ", name)
	cp, err := providers.GetProvider(name)
	if err != nil {
		logrus.Error(err)
		return
	}
	currProvider = cp
}

var providerCmd = &cobra.Command{
	Use:   "maas",
	Short: "target provider supported provider: maas",
	Long:  "target provider to fetch provider details and manage provider resources",
	Run: func(cmd *cobra.Command, args []string) {
		populateBMCredsFromCMD(cmd)
		cp, err := providers.GetProvider("maas")
		if err != nil {
			logrus.Error(err)
			fmt.Println(cmd.UsageString())
			return
		}
		currProvider = cp
	},
}

var listProvidersCmd = &cobra.Command{
	Use:   "list",
	Short: "list registered providers",
	Long:  "list registered providers",
	Run: func(cmd *cobra.Command, args []string) {
		populateBMCredsFromCMD(cmd)
		initProvider(cmd.Parent().Use)
		res := providers.GetProviders()
		for k, v := range res {
			fmt.Println(k, v)
		}
	},
}

var connectProviderCmd = &cobra.Command{
	Use:   "connect",
	Short: "connect to provider",
	Long:  "connect to provider",
	Run: func(cmd *cobra.Command, args []string) {
		populateBMCredsFromCMD(cmd)
		initProvider(cmd.Parent().Use)
		if currProvider == nil {
			logrus.Error("provider not found")
			fmt.Println(cmd.UsageString())
			return
		}
		err := currProvider.Connect(creds)
		if err != nil {
			logrus.Error(err)
			return
		}
		fmt.Printf("Connected to %s on %s\n", currProvider.WhoAmI(), creds.BaseURL)
	},
}

// var disconnectProviderCmd = &cobra.Command{
// 	Use:   "disconnect",
// 	Short: "disconnect from provider",
// 	Long:  "disconnect from provider",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		populateCredsFromCMD(cmd)
// 		providers := providers.Providers{}
// 		err := providers.Disconnect(Creds)
// 		if err != nil {
// 			logrus.Error(err)
// 		}
// 	},
// }

var setProviderPowerCmd = &cobra.Command{
	Use:   "set_power",
	Short: "set provider power",
	Long:  "set provider power",
	Run: func(cmd *cobra.Command, args []string) {
		populateBMCredsFromCMD(cmd)
		initProvider(cmd.Parent().Use)
		var machine_id string
		var action int32
		if val, err := cmd.Flags().GetString("machine_id"); err == nil {
			machine_id = val
		}
		if val, err := cmd.Flags().GetString("action"); err == nil {
			action = api.PowerStatus_value[val]
		}
		if currProvider == nil {
			logrus.Error("provider not found")
			fmt.Println(cmd.UsageString())
			return
		}
		err := currProvider.Connect(creds)
		if err != nil {
			logrus.Error(err)
		}
		err = currProvider.SetResourcePower(context.Background(), machine_id, api.PowerStatus(action))
		if err != nil {
			logrus.Error(err)
		}
	},
}

var listProviderResourcesCmd = &cobra.Command{
	Use:   "list_resources",
	Short: "list provider resources",
	Long:  "list provider resources",
	Run: func(cmd *cobra.Command, args []string) {
		populateBMCredsFromCMD(cmd)
		initProvider(cmd.Parent().Use)
		if currProvider == nil {
			logrus.Error("provider not found")
			fmt.Println(cmd.UsageString())
			return
		}
		err := currProvider.Connect(creds)
		if err != nil {
			logrus.Error(err)
		}

		logrus.Infof("Listing resources")
		res, err := currProvider.ListResources(context.Background())
		if err != nil {
			logrus.Error(err)
		}
		for k, v := range res {
			fmt.Println(k, v.Id, v.Fqdn, v.PowerState, v.Hostname)
		}
	},
}

var getResourceInfoCMD = &cobra.Command{
	Use:   "get_resource_info",
	Short: "get provider resource info",
	Long:  "get provider resource info",
	Run: func(cmd *cobra.Command, args []string) {
		populateBMCredsFromCMD(cmd)
		initProvider(cmd.Parent().Use)
		var resource_id string
		if val, err := cmd.Flags().GetString("resource_id"); err == nil {
			resource_id = val
		}
		if resource_id == "" {
			logrus.Error("resource_id is required")
			fmt.Println(cmd.UsageString())
			return
		}
		if currProvider == nil {
			logrus.Error("provider not found")
			fmt.Println(cmd.UsageString())
			return
		}
		err := currProvider.Connect(creds)
		if err != nil {
			logrus.Error(err)
		}

		logrus.Infof("Getting resource info")
		res, err := currProvider.GetResourceInfo(context.Background(), resource_id)
		if err != nil {
			logrus.Error(err)
		}
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			logrus.Errorf("Failed to marshal resource info: %v", err)
			return
		}
		fmt.Println(string(b))
	},
}

// var setProviderBootDeviceCmd = &cobra.Command{
// 	Use:   "set_boot_device",
// 	Short: "set provider boot device",
// 	Long:  "set provider boot device",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		populateBMCredsFromCMD(cmd)
// 		if currProvider == nil {
// 			logrus.Error("provider not found")
// 			fmt.Println(cmd.UsageString())
// 			return
// 		}
// 		err := currProvider.SetProviderBootDevice(creds)
// 		if err != nil {
// 			logrus.Error(err)
// 		}
// 	},
// }

func init() {
	rootCmd.AddCommand(providerCmd)
	providerCmd.PersistentFlags().StringP("api_key", "k", "", "Set the API key to use")
	providerCmd.PersistentFlags().StringP("base_url", "b", "", "Set the base URL to use")
	providerCmd.PersistentFlags().StringP("use_insecure", "i", "", "Set the datacenter to target")
	providerCmd.PersistentFlags().StringP("username", "U", "", "Set the username to use")
	providerCmd.PersistentFlags().StringP("password", "P", "", "Set the password to use")
	//set paramters for setPower
	setProviderPowerCmd.Flags().StringP("machine_id", "m", "", "Set the machine ID to use")
	setProviderPowerCmd.Flags().StringP("action", "a", "", "Set the power state to use")
	// get resource
	getResourceInfoCMD.Flags().StringP("resource_id", "r", "", "Set the resource ID to use")
	//add commands
	providerCmd.AddCommand(listProvidersCmd, connectProviderCmd, setProviderPowerCmd, listProviderResourcesCmd, getResourceInfoCMD)
}
