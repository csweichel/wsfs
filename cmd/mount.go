/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

var mountOpts struct {
	Daemonise bool
}

// mountCmd represents the mount command
var mountCmd = &cobra.Command{
	Use:  "mount",
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(mountCmd)

	mountCmd.PersistentFlags().BoolVar(&mountOpts.Daemonise, "daemonise", false, "Run process in the background")
}
