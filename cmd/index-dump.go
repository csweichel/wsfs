/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/csweichel/wsfs/pkg/idx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// indexDumpCmd represents the indexDump command
var indexDumpCmd = &cobra.Command{
	Use:   "dump <remoteIndexURL>",
	Short: "Dumps an entire index as JSON",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ridx, err := idx.OpenRemoteTarIndex(context.Background(), args[0])
		if err != nil {
			log.WithError(err).Fatal("cannot open index")
		}

		entries, err := ridx.RootEntries(context.Background())
		if err != nil {
			log.WithError(err).Fatal("cannot get root index")
		}

		var (
			res  []idx.Entry
			dirs []idx.Entry
		)
		for _, f := range entries {
			f := f
			res = append(res, f)
			if f.Dir() {
				dirs = append(dirs, f)
			}
		}

		for _, d := range dirs {
			children, err := ridx.Children(context.Background(), d)
			if err != nil {
				log.WithError(err).WithField("entry", d).Fatal("cannot get children")
			}
			for _, c := range children {
				c := c
				res = append(res, c)
				if c.Dir() {
					dirs = append(dirs, c)
				}
			}
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)
	},
}

func init() {
	indexCmd.AddCommand(indexDumpCmd)
}
