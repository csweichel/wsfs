/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/csweichel/wsfs/pkg/idxtar"
	"github.com/csweichel/wsfs/pkg/wsfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/sevlyar/go-daemon"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// mountRemoteCmd represents the mountRemote command
var mountRemoteCmd = &cobra.Command{
	Use:  "remote <baseURL> <mountpoint>",
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if daemon.WasReborn() {
			// let's go
		} else if mountOpts.Daemonise {
			ctx := daemon.Context{
				PidFileName: "/tmp/wsfs.pid",
				LogFileName: "/tmp/wsfs.log",
			}
			d, err := ctx.Reborn()
			if err != nil {
				log.WithError(err).Fatal("cannot daemonise")
			}
			if d != nil {
				return
			}

			log.Fatal("cannot daemonise")
		}

		t0 := time.Now()

		fsIndex, err := idxtar.OpenRemoteIndex(context.Background(), args[0])
		if err != nil {
			log.WithError(err).Fatal("cannot open remote index")
		}
		indexedRoot := wsfs.New(fsIndex)

		mnt := args[1]
		os.Mkdir(mnt, 0755)
		server, err := fs.Mount(mnt, indexedRoot, &fs.Options{
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
