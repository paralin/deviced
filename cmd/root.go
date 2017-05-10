package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fuserobotics/deviced/pkg/daemon"
	"github.com/spf13/cobra"
)

var configPath string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "deviced",
	Short: "Docker container management API and state machine.",
	Long:  `Manages the local docker daemon. Loads a configuration file. Config can be updated by the API.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		s := daemon.System{ConfigPath: configPath}
		os.Exit(s.Main())
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.

	RootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config path (default is /etc/deviced.yaml)")
}

func initConfig() {
	if configPath != "" {
		configPathAbs, err := filepath.Abs(configPath)
		if err != nil {
			fmt.Printf("Unable to format %s to absolute path, %s, using default path.\n", configPath, err)
			configPath = ""
		} else {
			configPath = configPathAbs
		}
	}

	if configPath == "" {
		configPath = "/etc/deviced.yaml"
	}
}
