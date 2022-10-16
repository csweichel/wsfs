/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

var mountOpts struct {
	Daemonise  bool
	DefaultGID uint32
	DefaultUID uint32
}

// mountCmd represents the mount command
var mountCmd = &cobra.Command{
	Use:  "mount",
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(mountCmd)

	mountCmd.PersistentFlags().BoolVar(&mountOpts.Daemonise, "daemonise", false, "Run process in the background")
	mountCmd.PersistentFlags().Uint32Var(&mountOpts.DefaultGID, "default-gid", 33333, "Default GID")
	mountCmd.PersistentFlags().Uint32Var(&mountOpts.DefaultUID, "default-uid", 33333, "Default UID")
}
