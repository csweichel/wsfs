package wsfs_test

import (
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/csweichel/wsfs/pkg/idx"
	"github.com/csweichel/wsfs/pkg/wsfs"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type mappedIndex []idx.Entry

type mappedFile struct {
	Nme      string
	Content  string
	Children []idx.Entry
	Attr     fuse.Attr
}

// Dir implements idx.Entry
func (mf *mappedFile) Dir() bool {
	return len(mf.Children) != 0
}

// Getattr implements idx.Entry
func (mf *mappedFile) Getattr(out *fuse.Attr) (applyDefaults bool, err error) {
	*out = mf.Attr
	return false, nil
}

// Mode implements idx.Entry
func (mf *mappedFile) StableMode() uint32 {
	if mf.Dir() {
		return syscall.S_IFDIR
	}
	return 0
}

// Name implements idx.Entry
func (mf *mappedFile) Name() string {
	return mf.Nme
}

// Read implements idx.Entry
func (mf *mappedFile) Read(dst []byte, offset int64) (n int, err error) {
	if offset >= int64(len(mf.Content)) {
		return 0, io.EOF
	}
	n = copy(dst, mf.Content[offset:])
	return
}

// Children implements idx.LazyIndex
func (mappedIndex) Children(ctx context.Context, of idx.Entry) ([]idx.Entry, error) {
	return of.(*mappedFile).Children, nil
}

// RootEntries implements idx.LazyIndex
func (mi mappedIndex) RootEntries(ctx context.Context) ([]idx.Entry, error) {
	return mi, nil
}

var _ idx.LazyIndex = ((mappedIndex)(nil))
var _ idx.Entry = ((*mappedFile)(nil))

func newDir(name string, children ...idx.Entry) idx.Entry {
	return &mappedFile{
		Nme:      name,
		Children: children,
		Attr: fuse.Attr{
			Mode: 0755,
			Owner: fuse.Owner{
				Uid: 33333,
				Gid: 33333,
			},
		},
	}
}

func newFile(name string, content string) idx.Entry {
	return &mappedFile{
		Nme:     name,
		Content: content,
		Attr: fuse.Attr{
			Size: uint64(len(content)),
			Mode: 0644,
			Owner: fuse.Owner{
				Uid: 33333,
				Gid: 33333,
			},
		},
	}
}

func TestCrawl(t *testing.T) {
	index := mappedIndex{
		newDir("d1",
			newFile("d1f1", "d1f1"),
			newFile("d1f2", "d1f2"),
			newFile("d1f3", "d1f3"),
			newDir("d1d1",
				newFile("d1d1f1", "d1d1f1"),
			),
		),
	}

	tempDir, err := os.MkdirTemp("", "test-crawl-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	root := wsfs.New(index, wsfs.Options{})
	server, err := fusefs.Mount(tempDir, root, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Unmount()

	filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, tempDir)

		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if exp := fs.FileMode(0755); stat.IsDir() && stat.Mode().Perm() != exp {
			t.Errorf("mode mismatch for %s: %o != %o", relPath, stat.Mode(), exp)
		} else if exp := fs.FileMode(0644); !stat.IsDir() && stat.Mode().Perm() != exp {
			t.Errorf("mode mismatch for %s: %v != %o", relPath, stat.Mode(), exp)
		}
		return nil
	})
}
