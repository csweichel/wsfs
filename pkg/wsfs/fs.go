package wsfs

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/csweichel/wsfs/pkg/idxtar"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	log "github.com/sirupsen/logrus"
)

type Options struct {
	DefaultUID, DefaultGID uint32
}

func New(index idxtar.Index, opts Options) fs.InodeEmbedder {
	return &indexedRoot{idx: index, opts: opts}
}

// indexedRoot is the root of the Zip filesystem. Its only functionality
// is populating the filesystem.
type indexedRoot struct {
	fs.Inode

	opts Options
	idx  idxtar.Index
}

// The root populates the tree in its OnAdd method
var _ fs.NodeOnAdder = (*indexedRoot)(nil)

func (zr *indexedRoot) OnAdd(ctx context.Context) {
	// OnAdd is called once we are attached to an Inode. We can
	// then construct a tree.  We construct the entire tree, and
	// we don't want parts of the tree to disappear when the
	// kernel is short on memory, so we use persistent inodes.
	if idx, ok := zr.idx.(idxtar.CompleteIndex); ok {
		for _, f := range idx.AllEntries() {
			dir, base := filepath.Split(f.Name())
			log.WithField("base", base).WithField("dir", dir).WithField("name", f.Name()).Debug("adding inode")

			p := &zr.Inode
			for _, component := range strings.Split(dir, "/") {
				if len(component) == 0 {
					continue
				}
				ch := p.GetChild(component)
				if ch == nil {
					ch = p.NewPersistentInode(ctx, &indexedFile{file: f, root: zr}, fs.StableAttr{Mode: fuse.S_IFDIR})
					p.AddChild(component, ch, true)
				}

				p = ch
			}
			if base == "" {
				continue
			}

			ch := p.NewPersistentInode(ctx, &indexedFile{file: f, root: zr}, fs.StableAttr{
				Mode: f.Mode(),
			})

			log.WithField("base", base).WithField("dir", dir).WithField("name", f.Name()).Debug("adding inode")
			p.AddChild(base, ch, true)
		}
		return
	}

	// OnAdd is called once we are attached to an Inode. We can
	// then construct a tree.  We construct the entire tree, and
	// we don't want parts of the tree to disappear when the
	// kernel is short on memory, so we use persistent inodes.
	if idx, ok := zr.idx.(idxtar.LazyIndex); ok {
		for _, f := range idx.RootEntries() {
			dir, base := filepath.Split(f.Name())
			log.WithField("base", base).WithField("dir", dir).WithField("name", f.Name()).Debug("adding inode")

			p := &zr.Inode
			ch := p.NewPersistentInode(ctx, &indexedFile{file: f, lazyIdx: idx, root: zr}, fs.StableAttr{
				Mode: f.Mode(),
			})

			log.WithField("base", base).WithField("dir", dir).WithField("name", f.Name()).Debug("adding inode")
			p.AddChild(base, ch, true)
		}
		return
	}
}

var _ fs.NodeGetattrer = (*indexedRoot)(nil)

func (zr *indexedRoot) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Gid = zr.opts.DefaultGID
	out.Uid = zr.opts.DefaultUID
	out.Mode = 0755 | syscall.S_IFDIR
	out.Size = 6
	return 0
}

// indexedFile is a file read from an indexed filesystem.
type indexedFile struct {
	fs.Inode
	file idxtar.Entry

	root    *indexedRoot
	lazyIdx idxtar.LazyIndex
}

// Getattr sets the minimum, which is the size. A more full-featured
// FS would also set timestamps and permissions.
var _ fs.NodeGetattrer = (*indexedFile)(nil)

func (zf *indexedFile) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	applyDefaults, err := zf.file.Getattr(&out.Attr)
	if err != nil {
		log.WithError(err).Warn("cannot getattr")
		return syscall.EINVAL
	}
	if applyDefaults {
		out.Gid = zf.root.opts.DefaultGID
		out.Uid = zf.root.opts.DefaultUID
	}
	return 0
}

var _ fs.NodeOpener = (*indexedFile)(nil)

func (zf *indexedFile) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	// // We don't return a filehandle since we don't really need
	// // one.  The file content is immutable, so hint the kernel to
	// // cache the data.
	// return nil, fuse.FOPEN_KEEP_CACHE, fs.OK
	return nil, 0, fs.OK
}

var _ fs.NodeReader = (*indexedFile)(nil)

// Read simply returns the data that was already unpacked in the Open call
func (zf *indexedFile) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := zf.file.Read(dest, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, syscall.EINVAL
	}

	return fuse.ReadResultData(dest[:n]), fs.OK
}

var _ fs.NodeReaddirer = (*indexedFile)(nil)

// Readdir implements fs.NodeReaddirer
func (zf *indexedFile) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if zf.lazyIdx == nil {
		return nil, 0
	}

	children, err := zf.lazyIdx.Children(ctx, zf.file)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.WithField("entry", zf.file).WithError(err).Warn("cannot load children")
		return nil, syscall.EINVAL
	}

	entries := make([]fuse.DirEntry, 0, len(children))
	for _, child := range children {
		entries = append(entries, fuse.DirEntry{
			Mode: child.Mode(),
			Name: child.Name(),
		})
	}

	return fs.NewListDirStream(entries), syscall.F_OK
}

var _ fs.NodeLookuper = (*indexedFile)(nil)

// Lookup implements fs.NodeLookuper
func (zf *indexedFile) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if zf.lazyIdx == nil {
		return nil, 0
	}

	children, err := zf.lazyIdx.Children(ctx, zf.file)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.WithField("entry", zf.file).WithError(err).Warn("cannot lookup file")
		return nil, syscall.EINVAL
	}

	var res idxtar.Entry
	for _, e := range children {
		if e.Name() == name {
			res = e
			break
		}
	}
	if res == nil {
		return nil, syscall.ENOENT
	}

	applyDefaults, err := res.Getattr(&out.Attr)
	if err != nil {
		log.WithError(err).Warn("cannot getattr on lookup")
		return nil, syscall.ENOENT
	}
	if applyDefaults {
		out.Gid = zf.root.opts.DefaultGID
		out.Uid = zf.root.opts.DefaultUID
	}

	log.WithField("name", name).WithField("res", res).WithField("attr", out.Attr).Debug("lookup file")

	return zf.NewPersistentInode(ctx, &indexedFile{
		file:    res,
		lazyIdx: zf.lazyIdx,
		root:    zf.root,
	}, fs.StableAttr{
		Mode: res.Mode(),
	}), fs.OK
}
