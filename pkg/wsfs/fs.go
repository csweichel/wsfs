package wsfs

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/csweichel/wsfs/pkg/idxtar"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/sirupsen/logrus"
)

func New(index idxtar.Index) fs.InodeEmbedder {
	return &zipRoot{idx: index}
}

// zipRoot is the root of the Zip filesystem. Its only functionality
// is populating the filesystem.
type zipRoot struct {
	fs.Inode

	idx idxtar.Index
}

// The root populates the tree in its OnAdd method
var _ fs.NodeOnAdder = (*zipRoot)(nil)

func (zr *zipRoot) OnAdd(ctx context.Context) {
	// OnAdd is called once we are attached to an Inode. We can
	// then construct a tree.  We construct the entire tree, and
	// we don't want parts of the tree to disappear when the
	// kernel is short on memory, so we use persistent inodes.
	for f := range zr.idx.Entries() {
		dir, base := filepath.Split(f.Name())

		p := &zr.Inode
		for _, component := range strings.Split(dir, "/") {
			if len(component) == 0 {
				continue
			}
			ch := p.GetChild(component)
			if ch == nil {
				ch = p.NewPersistentInode(ctx, &fs.Inode{}, fs.StableAttr{Mode: fuse.S_IFDIR})
				p.AddChild(component, ch, true)
			}

			p = ch
		}
		if base == "" {
			continue
		}

		ch := p.NewPersistentInode(ctx, &zipFile{file: f}, fs.StableAttr{
			Mode: f.Mode(),
		})

		logrus.WithField("base", base).WithField("dir", dir).WithField("name", f.Name()).Debug("adding inode")
		p.AddChild(base, ch, true)
	}
}

// zipFile is a file read from a zip archive.
type zipFile struct {
	fs.Inode
	file idxtar.File

	mu   sync.Mutex
	data []byte
}

// Getattr sets the minimum, which is the size. A more full-featured
// FS would also set timestamps and permissions.
var _ fs.NodeGetattrer = (*zipFile)(nil)

func (zf *zipFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	zf.file.Getattr(out)
	return 0
}

var _ fs.NodeOpener = (*zipFile)(nil)

func (zf *zipFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	// // We don't return a filehandle since we don't really need
	// // one.  The file content is immutable, so hint the kernel to
	// // cache the data.
	// return nil, fuse.FOPEN_KEEP_CACHE, fs.OK
	return nil, 0, fs.OK
}

var _ fs.NodeReader = (*zipFile)(nil)

// Read simply returns the data that was already unpacked in the Open call
func (zf *zipFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := zf.file.Read(dest, off, int64(len(dest)))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, syscall.EINVAL
	}

	return fuse.ReadResultData(dest[:n]), fs.OK
}
