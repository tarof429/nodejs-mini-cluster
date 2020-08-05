/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/spf13/cobra"
	nmc "github.com/tarof429/nmc"
)

// Flags
var site string
var count string
var port string
var serverPort string
var nginxVersion string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nginx-mini-cluster",
	Short: "A mini cluster using nginx",
	Long:  `A mini cluster using nginx.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {

		iCount, err := strconv.Atoi(count)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if iCount < 1 {
			fmt.Println("At least 1 proxy must be specified")
			os.Exit(1)
		}

		iPort, err := strconv.Atoi(port)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		serverPort, err := strconv.Atoi(serverPort)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		nmc.Run(site, iCount, serverPort, iPort, nginxVersion)

	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	//cobra.OnInitialize(initConfig)

	// Default values
	wd, _ := os.Getwd()
	site = path.Join(wd, "site")

	count = "2"

	port = "3001"

	serverPort = "3000"

	nginxVersion = "latest"

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nginx-mini-cluster.yaml)")

	rootCmd.PersistentFlags().StringVar(&site, "site", site, "Directory serving files")
	rootCmd.PersistentFlags().StringVar(&count, "count", count, "Number of reverse proxies")
	rootCmd.PersistentFlags().StringVar(&port, "port", port, "Initial port used by the proxies")
	rootCmd.PersistentFlags().StringVar(&serverPort, "server-port", serverPort, "Server port")
	rootCmd.PersistentFlags().StringVar(&nginxVersion, "nginx-version", nginxVersion, "nginx version")

	rootCmd.Version = "1.0"

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
// func initConfig() {
// 	if cfgFile != "" {
// 		// Use config file from the flag.
// 		viper.SetConfigFile(cfgFile)
// 	} else {
// 		// Find home directory.
// 		home, err := homedir.Dir()
// 		if err != nil {
// 			fmt.Println(err)
// 			os.Exit(1)
// 		}

// 		// Search config in home directory with name ".nginx-mini-cluster" (without extension).
// 		viper.AddConfigPath(home)
// 		viper.SetConfigName(".nginx-mini-cluster")
// 	}

// 	viper.AutomaticEnv() // read in environment variables that match

// 	// If a config file is found, read it in.
// 	if err := viper.ReadInConfig(); err == nil {
// 		fmt.Println("Using config file:", viper.ConfigFileUsed())
// 	}
// }
