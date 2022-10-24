package idx_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/csweichel/wsfs/pkg/idx"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const (
	fileHelloTXT       = "Hello World\nThis is a test"
	fileHidden         = "Filename starts with a ."
	fileFooSlashBarTXT = "More file content"
)

func TestChildren(t *testing.T) {
	type Expectation struct {
		Entries []string
		Err     string
	}
	tests := []struct {
		Name        string
		Index       idx.LazyIndex
		Path        string
		Expectation Expectation
	}{
		{
			Name:  "foo",
			Index: prepareTestIndex(t),
			Path:  "foo",
			Expectation: Expectation{
				Entries: []string{"bar.txt:644", "dir:755", "three:755"},
			},
		},
		{
			Name:  "foo/three",
			Index: prepareTestIndex(t),
			Path:  "foo/three",
			Expectation: Expectation{
				Entries: []string{"levels:755"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var e idx.Entry
			root, err := test.Index.RootEntries(context.Background())
			if err != nil {
				t.Fatalf("cannot get root entries: %v", err)
			}
			for i, s := range strings.Split(test.Path, "/") {
				if i > 0 {
					root, err = test.Index.Children(context.Background(), e)
					if err != nil {
						t.Fatalf("cannot get child entries: %v", err)
					}
				}

				for _, r := range root {
					if r.Name() == s {
						e = r
						break
					}
				}
				if e == nil {
					t.Fatalf("did not find entry named %s", test.Path)
				}
			}
			if e == nil {
				t.Fatalf("did not find entry named %s", test.Path)
			}
			t.Logf("found entry named %s", e.Name())

			var act Expectation
			res, err := test.Index.Children(context.Background(), e)
			if err != nil {
				act.Err = err.Error()
			}

			act.Entries = make([]string, 0, len(res))
			for _, e := range res {
				var attr fuse.Attr
				e.Getattr(&attr)
				act.Entries = append(act.Entries, fmt.Sprintf("%s:%o", e.Name(), attr.Mode))
			}

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("Children() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRootEntries(t *testing.T) {
	type Expectation struct {
		Entries []string
		Err     string
	}
	tests := []struct {
		Name        string
		Index       idx.LazyIndex
		Expectation Expectation
	}{
		{
			Name:  "happy path",
			Index: prepareTestIndex(t),
			Expectation: Expectation{
				Entries: []string{"foo", "hello.txt", "hidden"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var act Expectation
			res, err := test.Index.RootEntries(context.Background())
			if err != nil {
				act.Err = err.Error()
			}

			act.Entries = make([]string, 0, len(res))
			for _, e := range res {
				act.Entries = append(act.Entries, e.Name())
			}

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("RootEntries() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func prepareTestIndex(t *testing.T) idx.LazyIndex {
	buf := bytes.NewBuffer(nil)

	tarw := tar.NewWriter(buf)
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "./", Mode: 0755, Uid: 33333, Gid: 33333})
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "./hello.txt", Mode: 0644, Uid: 33333, Gid: 33333, Size: int64(len(fileHelloTXT))})
	tarw.Write([]byte(fileHelloTXT))
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "./hidden", Mode: 0644, Uid: 33333, Gid: 33333, Size: int64(len(fileHidden))})
	tarw.Write([]byte(fileHidden))
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "./foo/", Mode: 0755, Uid: 33333, Gid: 33333})
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "./foo/bar.txt", Mode: 0644, Uid: 33333, Gid: 33333, Size: int64(len(fileFooSlashBarTXT))})
	tarw.Write([]byte(fileFooSlashBarTXT))
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "./foo/dir", Mode: 0755, Uid: 33333, Gid: 33333})
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "./foo/three", Mode: 0755, Uid: 33333, Gid: 33333})
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "./foo/three/levels", Mode: 0755, Uid: 33333, Gid: 33333})
	tarw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "./foo/three/levels/deep", Mode: 0755, Uid: 33333, Gid: 33333, Size: int64(len(fileHelloTXT))})
	tarw.Write([]byte(fileHelloTXT))
	tarw.Close()

	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
	if err != nil {
		t.Fatal(err)
	}
	err = idx.ProduceIndexFromTarFile(db, buf)
	if err != nil {
		t.Fatal(err)
	}
	res, err := idx.OpenTarIndex(db, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return res
}
