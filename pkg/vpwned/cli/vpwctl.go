package cli

import (
	_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/base"
	_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/maas"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var Config config

type config struct {
	Host    string
	Port    string
	APIPort string
}

func (c *config) ParseConfig() {
	c.Host = viper.GetString("host")        // default: localhost
	c.Port = viper.GetString("port")        //default: 3000
	c.APIPort = viper.GetString("api_port") //default: 3001
}

var rootCmd = &cobra.Command{
	Use:   "vpwctl",
	Short: "start the vpwctl server",
}

func Execute() error {
	return rootCmd.Execute()
}
