package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
)

var kubeconfigArgs = genericclioptions.NewConfigFlags(false)

var logLevel string

var rootCmd = &cobra.Command{
	Use:   "fluxwh",
	Short: "Helper tool to automate the creation of flux webhooks",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level")
	kubeconfigArgs.AddFlags(rootCmd.PersistentFlags())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
