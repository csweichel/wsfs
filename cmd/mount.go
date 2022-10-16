/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// mountCmd represents the mount command
var mountCmd = &cobra.Command{
	Use:  "mount",
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(mountCmd)
}
