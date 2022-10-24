package cmd

import (
	"os"

	"github.com/csweichel/wsfs/pkg/idx"
	"github.com/dgraph-io/badger/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// indexGenerateCmd represents the indexGenerate command
var indexGenerateCmd = &cobra.Command{
	Use:   "generate <dst> <src.tar>",
	Short: "Generate an index from a tar file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		in, err := os.Open(args[1])
		if err != nil {
			log.WithError(err).Fatal("cannot open source file")
		}
		defer in.Close()

		db, err := badger.Open(badger.DefaultOptions(args[0]))
		if err != nil {
			log.WithError(err).Fatal("cannot open database")
		}
		defer db.Close()

		err = idx.ProduceIndexFromTarFile(db, in)
		if err != nil {
			log.WithError(err).Fatal("cannot produce index")
		}
	},
}

func init() {
	indexCmd.AddCommand(indexGenerateCmd)
}
