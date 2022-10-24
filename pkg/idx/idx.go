package idx

import (
	"context"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type Index interface{}

type LazyIndex interface {
	RootEntries(ctx context.Context) ([]Entry, error)
	Children(ctx context.Context, of Entry) ([]Entry, error)
}

type Entry interface {
	Name() string
	Dir() bool
	Getattr(out *fuse.Attr) (applyDefaults bool, err error)

	// Mode is used on the stableAttr of the inode
	StableMode() uint32

	Read(dst []byte, offset int64) (n int, err error)
}
