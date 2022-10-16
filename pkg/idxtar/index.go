package idxtar

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/hanwen/go-fuse/v2/fuse"
	log "github.com/sirupsen/logrus"
	"github.com/snabb/httpreaderat"
)

func OpenRemoteIndex(ctx context.Context, baseURL string) (Index, error) {
	tmpdir, err := os.MkdirTemp("", "wsfs-index-*")
	if err != nil {
		return nil, err
	}

	// download the index
	idxDlStart := time.Now()
	var timeout time.Duration
	if dl, ok := ctx.Deadline(); ok {
		timeout = time.Until(dl)
	}
	client := &http.Client{
		Timeout: timeout,
	}
	res, err := client.Get(baseURL + ".index")
	if err != nil {
		return nil, fmt.Errorf("cannot download index: %v", err)
	}
	defer res.Body.Close()

	gzipR, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, err
	}
	defer gzipR.Close()
	tarR := tar.NewReader(gzipR)

	err = extractTarTo(tmpdir, tarR)
	if err != nil {
		return nil, err
	}
	log.WithField("tmpdir", tmpdir).WithField("duration", time.Since(idxDlStart)).Debug("downloaded index")

	idx, err := badger.Open(badger.DefaultOptions(tmpdir))
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", baseURL+".tar", nil)
	htrdr, err := httpreaderat.New(nil, req, httpreaderat.NewDefaultStore())
	if err != nil {
		return nil, err
	}

	return &fileBackedIndex{
		TarFile: htrdr,
		Index:   idx,
	}, nil
}

func extractTarTo(dst string, tr *tar.Reader) error {
	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

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
func (e *fileBackedIndexEntry) Read(dst []byte, offset int64, len int64) (n int, err error) {
	return e.TarFile.ReadAt(dst[:len], e.Entry.Offset+offset)
}

// Size implements File
func (e *fileBackedIndexEntry) Getattr(out *fuse.AttrOut) error {
	hdr := e.Entry.TarHeader

	out.Atime = uint64(hdr.AccessTime.Unix())
	out.Gid = uint32(hdr.Gid)
	out.Mode = uint32(hdr.Mode)
	out.Mtime = uint64(hdr.ModTime.Unix())
	out.Size = uint64(hdr.Size)
	out.Uid = uint32(hdr.Uid)

	return nil
}

func (e *fileBackedIndexEntry) Mode() uint32 {
	switch e.Entry.TarHeader.Typeflag {
	case tar.TypeSymlink:
		return syscall.S_IFLNK

	case tar.TypeLink:
		log.Warn("don't know how to handle Typelink")
		return 0

	case tar.TypeChar:
		return syscall.S_IFCHR
	case tar.TypeBlock:
		return syscall.S_IFBLK
	case tar.TypeDir:
		return syscall.S_IFDIR
	case tar.TypeFifo:
		return syscall.S_IFIFO
	case tar.TypeReg, tar.TypeRegA:
		return 0
	default:
		return 0
	}
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
	Getattr(out *fuse.AttrOut) error

	// Mode is used on the stableAttr of the inode
	Mode() uint32

	Read(dst []byte, offset int64, len int64) (n int, err error)
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
