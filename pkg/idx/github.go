package idx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/shurcooL/githubv4"
	log "github.com/sirupsen/logrus"
	"github.com/snabb/httpreaderat"
	"golang.org/x/oauth2"
)

func NewGitHubIndex(ctx context.Context, ghToken, owner, repo, revision string) (LazyIndex, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: ghToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	res := &githubIndex{
		Client:     client,
		HTTPClient: httpClient,
		Owner:      owner,
		Repo:       repo,
		Revision: revision,
	}

	err := res.fetchRoot(ctx)
	if err != nil {
		return nil, err
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		go func() {
			var cnt int
			for {
				time.Sleep(5 * time.Second)

				res.mu.RLock()
				if cnt == len(res.children) {
					res.mu.RUnlock()
					continue
				}
				cnt = len(res.children)
				_ = json.NewEncoder(os.Stdout).Encode(res.children)
				res.mu.RUnlock()
			}
		}()
	}

	return res, nil
}

var _ Index = (*githubIndex)(nil)

type githubIndex struct {
	Client     *githubv4.Client
	HTTPClient *http.Client
	Owner      string
	Repo       string
	Revision   string

	children map[string][]*githubEntry
	mu       sync.RWMutex
}

func (n *githubIndex) fetch(ctx context.Context, path string) ([]*githubEntry, error) {
	var query struct {
		Repository struct {
			Object struct {
				Tree struct {
					Entries []struct {
						Name   githubv4.String
						Type   githubv4.String
						Path   githubv4.String
						Mode   githubv4.Int
						Object struct {
							Blob struct {
								ByteSize githubv4.Int
							} `graphql:"... on Blob"`
						}
					}
				} `graphql:"... on Tree"`
			} `graphql:"object(expression: $expr)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]interface{}{
		"owner": githubv4.String(n.Owner),
		"name":  githubv4.String(n.Repo),
		"expr":  githubv4.String(n.Revision + ":" + path),
	}

	log.WithField("vars", vars).Debug("fetching entries")
	t0 := time.Now()
	err := n.Client.Query(ctx, &query, vars)
	if err != nil {
		return nil, err
	}
	log.WithField("duration", time.Since(t0)).Debug("done fetching entries")

	children := make([]*githubEntry, 0, len(query.Repository.Object.Tree.Entries))
	for _, entry := range query.Repository.Object.Tree.Entries {
		child := &githubEntry{
			idx:      n,
			Fullpath: string(entry.Path),
			Nme:      string(entry.Name),
			Mde:      uint32(entry.Mode),
			Tree:     entry.Type == "tree",
			Sze:      uint64(entry.Object.Blob.ByteSize),
		}
		log.WithField("child", child).WithField("entry", entry).Debug("adding entry")
		children = append(children, child)
	}
	return children, nil
}

func (n *githubIndex) fetchRoot(ctx context.Context) error {
	res, err := n.fetch(ctx, "")
	if err != nil {
		return fmt.Errorf("cannot fetch root: %w", err)
	}
	n.children = map[string][]*githubEntry{
		"": res,
	}

	return nil
}

// Entries implements Index
func (n *githubIndex) RootEntries(ctx context.Context) ([]Entry, error) {
	res := make([]Entry, 0, len(n.children))
	for _, c := range n.children[""] {
		res = append(res, c)
	}
	return res, nil
}

func (n *githubIndex) Children(ctx context.Context, of Entry) ([]Entry, error) {
	
	entry := of.(*githubEntry)
	n.mu.RLock()
	children, ok := n.children[entry.Fullpath]
	n.mu.RUnlock()
	if !ok {
		log.WithField("of", of).Debug("getting children")

		var err error
		children, err = n.fetch(ctx, entry.Fullpath)
		if err != nil {
			return nil, err
		}

		n.mu.Lock()
		n.children[entry.Fullpath] = children
		n.mu.Unlock()
	}

	res := make([]Entry, len(children))
	for i := range children {
		res[i] = children[i]
	}
	return res, nil
}

var _ Entry = (*githubEntry)(nil)

type githubEntry struct {
	idx *githubIndex

	ID       string
	Fullpath string
	Nme      string
	Tree     bool
	Sze      uint64
	Mde      uint32

	mu sync.Mutex
	r  *httpreaderat.HTTPReaderAt
}

func (e *githubEntry) Dir() bool {
	return e.Tree
}

// Getattr implements File
func (e *githubEntry) Getattr(out *fuse.Attr) (applyDefaults bool, err error) {
	out.Size = e.Sze
	out.Mode = e.Mde

	return true, nil
}

// Mode implements File
func (e *githubEntry) StableMode() uint32 {
	if e.Tree {
		return syscall.S_IFDIR
	}
	return syscall.S_IFREG
}

// Name implements File
func (e *githubEntry) Name() string {
	return e.Nme
}

// Read implements File
func (e *githubEntry) Read(dst []byte, offset int64) (n int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.r == nil {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://github.com/%s/%s/raw/%s/%s", e.idx.Owner, e.idx.Repo, e.idx.Revision, e.Fullpath), nil)
		if err != nil {
			return 0, err
		}

		e.r, err = httpreaderat.New(e.idx.HTTPClient, req, nil)
		if err != nil {
			return 0, err
		}
	}

	return e.r.ReadAt(dst, offset)
}
