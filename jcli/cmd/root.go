/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"jcli/auth"
	"jcli/jenkins"

	"github.com/spf13/cobra"
)

var Address string
var User string
var Jenkins *jenkins.Jenkins

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jcli",
	Short: "A brief description of your application",
	Long:  `Long description of your application. This is where you would put`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
}

// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Execute the root command
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func InitJenkins() {
	apiKey := auth.LoadAPIKeyfromKeyring(Address, User)
	Jenkins = jenkins.NewJenkins(Address, User, apiKey)
}

func init() {
	// Always init Jenkins before running any command
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.jcli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.PersistentFlags().StringVarP(&Address, "address", "a", "", "Address of Jenkins server.")
	rootCmd.MarkFlagRequired("address")
	rootCmd.PersistentFlags().StringVarP(&User, "user", "u", "", "User to connect to Jenkins server.")
	rootCmd.MarkFlagRequired("user")
}
