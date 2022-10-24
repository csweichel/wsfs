/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/csweichel/wsfs/pkg/idx"
	"github.com/csweichel/wsfs/pkg/wsfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// mountLocalCmd represents the mountLocal command
var mountLocalCmd = &cobra.Command{
	Use: "local <path/to/index> <path/to/tar> <mountpoint>",
	Run: func(cmd *cobra.Command, args []string) {
		t0 := time.Now()

		fsIndex, err := idx.OpenFileBackedTarIndex(args[0], args[1])
		if err != nil {
			logrus.WithError(err).Fatal("cannot open indexed tar")
		}

		root := wsfs.New(fsIndex, wsfs.Options{
			DefaultUID: mountOpts.DefaultUID,
			DefaultGID: mountOpts.DefaultGID,
		})

		mnt := args[2]
		os.Mkdir(mnt, 0755)
		server, err := fs.Mount(mnt, root, &fs.Options{
			MountOptions: fuse.MountOptions{
				Debug:      rootOpts.Verbose,
				AllowOther: mountOpts.AllowOther,
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mounted in %v\n", time.Since(t0))
		fmt.Printf("to unmount: fusermount -u %s\n", mnt)
		server.Wait()
	},
}

func init() {
	mountCmd.AddCommand(mountLocalCmd)
}
