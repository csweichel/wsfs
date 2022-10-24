/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/csweichel/wsfs/pkg/idx"
	"github.com/csweichel/wsfs/pkg/wsfs"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var mountGithubOpts struct {
	Revision string
}

// mountGithubCmd represents the mountGithub command
var mountGithubCmd = &cobra.Command{
	Use:   "github <owner/repo> <mountpoint>",
	Args:  cobra.ExactArgs(2),
	Short: "Mounts a GitHub repo as filesystem. Use $GITHUB_TOKEN to pass in the token.",
	Run: func(cmd *cobra.Command, args []string) {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			log.Fatal("missing $GITHUB_TOKEN environment variable")
		}

		segs := strings.Split(args[0], "/")
		if len(segs) != 2 {
			log.WithField("segments", segs).Fatal("invalid repo format - must be owner/repo")
		}
		owner, repo := segs[0], segs[1]

		idx, err := idx.NewGitHubIndex(context.Background(), token, owner, repo, mountGithubOpts.Revision)
		if err != nil {
			log.WithError(err).Fatal("cannot build GitHub index")
		}

		idxfs := wsfs.New(idx, wsfs.Options{
			DefaultUID: mountOpts.DefaultUID,
			DefaultGID: mountOpts.DefaultGID,
		})

		mnt := args[1]
		os.Mkdir(mnt, 0755)
		server, err := fs.Mount(mnt, idxfs, &fs.Options{
			MountOptions: fuse.MountOptions{
				Debug: rootOpts.Verbose,
			},
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("to unmount: fusermount -u %s\n", mnt)
		server.Wait()
	},
}

func init() {
	mountCmd.AddCommand(mountGithubCmd)
	mountGithubCmd.Flags().StringVar(&mountGithubOpts.Revision, "revision", "main", "Revision to serve")
}
