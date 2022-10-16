/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/csweichel/wsfs/pkg/idxtar"
	"github.com/csweichel/wsfs/pkg/wsfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// mountRemoteCmd represents the mountRemote command
var mountRemoteCmd = &cobra.Command{
	Use:  "remote <baseURL> <mountpoint>",
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		t0 := time.Now()

		fsIndex, err := idxtar.OpenRemoteIndex(context.Background(), args[0])
		if err != nil {
			logrus.WithError(err).Fatal("cannot open remote index")
		}

		root := wsfs.New(fsIndex)

		mnt := args[1]
		os.Mkdir(mnt, 0755)
		server, err := fs.Mount(mnt, root, &fs.Options{
			MountOptions: fuse.MountOptions{
				Debug: rootOpts.Verbose,
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
	mountCmd.AddCommand(mountRemoteCmd)
}
