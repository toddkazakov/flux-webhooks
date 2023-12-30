package cmd

import "github.com/spf13/cobra"

var dockerhubCmd = &cobra.Command{
	Use:   "dockerhub",
	Short: "Docker Hub Webhooks",
	Long:  "The dockerhub command is used to manage dockerhub webhooks",
}

func init() {
	rootCmd.AddCommand(dockerhubCmd)
}
