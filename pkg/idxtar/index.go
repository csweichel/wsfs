package idxtar

import (
	"archive/tar"
	"encoding/json"
	"io"
	"os"
	"strings"

	badger "github.com/dgraph-io/badger/v3"
	log "github.com/sirupsen/logrus"
)

func OpenFileBackedIndex(index, tarfile string) (Index, error) {
	idx, err := badger.Open(badger.DefaultOptions(index))
	if err != nil {
		return nil, err
	}
	tarf, err := os.Open(tarfile)
	if err != nil {
		return nil, err
	}

	return &fileBackedIndex{
		TarFile: tarf,
		Index:   idx,
	}, nil
}

type fileBackedIndex struct {
	TarFile io.ReaderAt
	Index   *badger.DB
}

func (fs *fileBackedIndex) Entries() <-chan File {
	res := make(chan File)
	go func() {
		defer close(res)

		_ = fs.Index.View(func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()

				err := item.Value(func(v []byte) error {
					var e indexEntry
					err := json.Unmarshal(v, &e)
					if err != nil {
						return err
					}

					res <- &fileBackedIndexEntry{
						TarFile: fs.TarFile,
						Entry:   e,
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	}()
	return res
}

type fileBackedIndexEntry struct {
	TarFile io.ReaderAt
	Entry   indexEntry
}

// Read implements File
func (*fileBackedIndexEntry) Read(dst []byte, offset int64, len int64) {

}

// Size implements File
func (e *fileBackedIndexEntry) Size() uint64 {
	return uint64(e.Entry.TarHeader.Size)
}

// Name implements File
func (e *fileBackedIndexEntry) Name() string {
	return e.Entry.TarHeader.Name
}

var _ File = (*fileBackedIndexEntry)(nil)

type Index interface {
	Entries() <-chan File
}

type File interface {
	Name() string
	Size() uint64

	Read(dst []byte, offset int64, len int64)
}

type indexEntry struct {
	Offset    int64
	TarHeader *tar.Header
}

func ProduceIndexFromTarFile(dst string, in io.Reader) error {
	db, err := badger.Open(badger.DefaultOptions(dst))
	if err != nil {
		return err
	}
	defer db.Close()

	indexingR := &indexingReader{
		Reader: in,
	}

	tarf := tar.NewReader(indexingR)
	for hdr, err := tarf.Next(); err == nil; hdr, err = tarf.Next() {
		hdr.Name = strings.TrimPrefix(hdr.Name, "./")

		hdrJson, err := json.Marshal(indexEntry{
			Offset:    indexingR.Offset,
			TarHeader: hdr,
		})
		if err != nil {
			return err
		}

		db.Update(func(txn *badger.Txn) error {
			err := txn.Set([]byte(hdr.Name), hdrJson)
			if err != nil {
				return err
			}
			return txn.Commit()
		})
		log.WithField("name", hdr.Name).WithField("offset", indexingR.Offset).Debug("added file to index")
	}
	_ = db.Flatten(5)

	return nil
}

type indexingReader struct {
	io.Reader

	Offset int64
}

func (r *indexingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.Offset += int64(n)
	return n, err
}

type IndexEntry struct {
	Offset int
	Size   int
}
