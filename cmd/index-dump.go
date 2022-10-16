/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/csweichel/wsfs/pkg/idxtar"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// indexDumpCmd represents the indexDump command
var indexDumpCmd = &cobra.Command{
	Use:   "dump <remoteIndexURL>",
	Short: "Dumps an entire index as JSON",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		idx, err := idxtar.OpenRemoteIndex(context.Background(), args[0])
		if err != nil {
			log.WithError(err).Fatal("cannot open index")
		}

		var res []idxtar.Entry
		for _, f := range idx.AllEntries() {
			f := f
			res = append(res, f)
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)
	},
}

func init() {
	indexCmd.AddCommand(indexDumpCmd)
}
