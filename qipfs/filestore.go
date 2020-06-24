package qipfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	// Note coreunix is forked form github.com/ipfs/go-ipfs/core/coreunix
	// we need coreunix.Adder.addFile to be exported to get access to dags while
	// they're being created. We should be able to remove this with refactoring &
	// moving toward coreapi.coreUnix().Add() with properly-configured options,
	// but I'd like a test before we do that. We may also want to consider switching
	// Qri to writing IPLD. Lots to think about.
	coreunix "github.com/qri-io/qfs/qipfs/coreunix"

	"github.com/ipfs/go-cid"
	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	ipfsrepo "github.com/ipfs/go-ipfs/repo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/qri-io/qfs"
	cafs "github.com/qri-io/qfs/cafs"
	files "github.com/qri-io/qfs/qipfs/go-ipfs-files"
	"github.com/qri-io/qfs/qipfs/qipfs_http"
)

var (
	log = logging.Logger("qipfs")
	// ErrNoRepoPath is returned when no repo path is provided in the config
	ErrNoRepoPath = errors.New("must provide a repo path ('path') to initialize an ipfs filesystem")
)

type Filestore struct {
	cfg  *StoreCfg
	node *core.IpfsNode
	capi coreiface.CoreAPI

	doneCh  chan struct{}
	doneErr error
}

var (
	_ qfs.ReleasingFilesystem = (*Filestore)(nil)
	_ cafs.Fetcher            = (*Filestore)(nil)
)

// NewFilesystem creates a new local filesystem PathResolver
// with no options
func NewFilesystem(ctx context.Context, cfgMap map[string]interface{}) (qfs.Filesystem, error) {
	cfg, err := mapToConfig(cfgMap)
	if err != nil {
		return nil, err
	}

	if cfg.BuildCfg.ExtraOpts == nil {
		cfg.BuildCfg.ExtraOpts = map[string]bool{}
	}
	cfg.BuildCfg.ExtraOpts["pubsub"] = cfg.EnablePubSub

	if cfg.Path == "" && cfg.URL == "" {
		return nil, ErrNoRepoPath
	} else if cfg.URL != "" {
		return qipfs_http.NewFilesystem(map[string]interface{}{"url": cfg.URL})
	}

	if err := LoadIPFSPluginsOnce(cfg.Path); err != nil {
		return nil, err
	}

	cfg.Repo, err = openRepo(ctx, cfg)
	if err != nil {
		if cfg.URL != "" && err == errRepoLock {
			// if we cannot get a repo, and we have a fallback APIAdder
			// attempt to create and return an `qipfs_http` filesystem istead
			return qipfs_http.NewFilesystem(map[string]interface{}{"url": cfg.URL})
		}
		log.Errorf("opening %q: %s", cfg.Path, err)
		return nil, err
	}

	node, err := core.NewNode(ctx, &cfg.BuildCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating ipfs node: %s", err.Error())
	}

	capi, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return nil, err
	}

	fst := &Filestore{
		cfg:    cfg,
		node:   node,
		capi:   capi,
		doneCh: make(chan struct{}),
	}

	go func(fst *Filestore) {
		<-ctx.Done()
		fst.doneErr = ctx.Err()
		log.Debugf("closing repo at %q", cfg.Path)
		if err := cfg.Repo.Close(); err != nil {
			log.Error(err)
		}
		for {
			daemonLocked, err := fsrepo.LockedByOtherProcess(cfg.Path)
			if err != nil {
				log.Error(err)
				break
			} else if daemonLocked {
				log.Errorf("fsrepo is still locked")
				time.Sleep(time.Millisecond * 25)
				continue
			}
			break
		}
		log.Debugf("closed repo at %q", cfg.Path)
		close(fst.doneCh)
	}(fst)

	return fst, nil
}

// NewFilesystemFromNode wraps an existing IPFS node with a qfs.Filesystem
func NewFilesystemFromNode(node *core.IpfsNode) (qfs.Filesystem, error) {
	capi, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return nil, err
	}

	return &Filestore{
		node: node,
		capi: capi,
	}, nil
}

// FilestoreType uniquely identifies this filestore
const FilestoreType = "ipfs"

// Type distinguishes this filesystem from others by a unique string prefix
func (fst Filestore) Type() string {
	return FilestoreType
}

// Done implements the qfs.ReleasingFilesystem interface
func (fst *Filestore) Done() <-chan struct{} {
	return fst.doneCh
}

// DoneErr returns errors in closing the filesystem
func (fst *Filestore) DoneErr() error {
	return fst.doneErr
}

// Node exposes the internal ipfs node
//
// Deprecated: use IPFSCoreAPI instead
func (fst *Filestore) Node() *core.IpfsNode {
	return fst.node
}

// IPFSCoreAPI exposes the Filestore's CoreAPI interface
func (fst *Filestore) IPFSCoreAPI() coreiface.CoreAPI {
	return fst.capi
}

func openRepo(ctx context.Context, cfg *StoreCfg) (ipfsrepo.Repo, error) {
	if cfg.NilRepo {
		return nil, nil
	}
	if cfg.Repo != nil {
		return nil, nil
	}
	if cfg.Path != "" {
		log.Debugf("opening repo at %q", cfg.Path)
		if daemonLocked, err := fsrepo.LockedByOtherProcess(cfg.Path); err != nil {
			return nil, err
		} else if daemonLocked {
			return nil, errRepoLock
		}
		localRepo, err := fsrepo.Open(cfg.Path)
		if err != nil {
			if err == fsrepo.ErrNeedMigration {
				return nil, ErrNeedMigration
			}
			return nil, fmt.Errorf("error opening local filestore ipfs repository: %w", err)
		}

		return localRepo, nil
	}
	return nil, fmt.Errorf("no repo path to open IPFS fsrepo")
}

func (fst *Filestore) Online() bool {
	return fst.node.IsOnline
}

func (fst *Filestore) GoOnline(ctx context.Context) error {
	log.Debug("going online")
	cfg := fst.cfg
	cfg.BuildCfg.Online = true
	node, err := core.NewNode(ctx, &cfg.BuildCfg)
	if err != nil {
		return fmt.Errorf("error creating ipfs node: %s\n", err.Error())
	}

	capi, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}

	*fst = Filestore{
		cfg:  cfg,
		node: node,
		capi: capi,

		doneCh:  fst.doneCh,
		doneErr: fst.doneErr,
	}

	if cfg.EnableAPI {
		go func() {
			if err := fst.serveAPI(); err != nil {
				log.Errorf("error serving IPFS HTTP api: %s", err)
			}
		}()
	}

	return nil
}

func (fst *Filestore) Has(ctx context.Context, key string) (exists bool, err error) {
	id, err := cid.Parse(key)
	if err != nil {
		return false, err
	}
	return fst.node.Blockstore.Has(id)
}

func (fst *Filestore) Get(ctx context.Context, key string) (qfs.File, error) {
	return fst.getKey(ctx, key)
}

func (fst *Filestore) Fetch(ctx context.Context, source cafs.Source, key string) (qfs.File, error) {
	return fst.getKey(ctx, key)
}

// Put adds a file and pins
func (fst *Filestore) Put(ctx context.Context, file qfs.File) (key string, err error) {
	hash, err := fst.AddFile(file, true)
	if err != nil {
		log.Infof("error adding bytes: %s", err.Error())
		return
	}
	return pathFromHash(hash), nil
}

func (fst *Filestore) Delete(ctx context.Context, key string) error {
	err := fst.Unpin(ctx, key, true)
	if err != nil {
		if err.Error() == "not pinned" {
			return nil
		}
	}
	return nil
}

func (fst *Filestore) getKey(ctx context.Context, key string) (qfs.File, error) {
	node, err := fst.capi.Unixfs().Get(ctx, path.New(key))
	if err != nil {
		return nil, err
	}

	if rdr, ok := node.(io.ReadCloser); ok {
		return ipfsFile{path: key, r: rdr}, nil
	}

	// if _, isDir := node.(files.Directory); isDir {
	// 	return nil, fmt.Errorf("filestore doesn't support getting directories")
	// }

	return nil, fmt.Errorf("path is neither a file nor a directory")
}

// Adder wraps a coreunix adder to conform to the cafs adder interface
type Adder struct {
	adder *coreunix.Adder
	out   chan interface{}
	added chan cafs.AddedFile
	wrap  bool
	pin   bool
}

func (a *Adder) AddFile(ctx context.Context, f qfs.File) error {
	return a.adder.AddFile(wrapFile{f})
}

func (a *Adder) Added() chan cafs.AddedFile {
	return a.added
}

func (a *Adder) Close() error {
	defer close(a.out)
	// node, err := a.adder.CurRootNode()
	// if err != nil {
	// 	return err
	// }
	// if a.wrap {
	// 	// rootDir := files.NewSliceDirectory([]files.DirEntry{
	// 	// 	files.FileEntry("", files.ToFile(node)),
	// 	// })
	// 	// if err := a.adder.AddDir("", rootDir, true); err != nil {
	// 	// 	return err
	// 	// }
	// 	node, err = a.adder.RootDirectory()
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	if _, err := a.adder.Finalize(); err != nil {
		return err
	}

	if a.pin {
		return a.adder.PinRoot()
	}

	return nil
}

func (fst *Filestore) NewAdder(pin, wrap bool) (cafs.Adder, error) {
	node := fst.node
	ctx := context.Background()

	a, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, node.DAG)
	if err != nil {
		return nil, fmt.Errorf("error allocating adder: %s", err.Error())
	}

	outChan := make(chan interface{}, 9)
	added := make(chan cafs.AddedFile, 9)
	a.Out = outChan
	a.Pin = pin
	a.Wrap = wrap

	go func() {
		for {
			select {
			case out, ok := <-outChan:
				if ok {
					output := out.(*coreunix.AddEvent)
					if output.Hash != "" {
						added <- cafs.AddedFile{
							Path:  pathFromHash(output.Hash),
							Name:  output.Name,
							Bytes: output.Bytes,
							Size:  output.Size,
						}
					}
				} else {
					close(added)
					return
				}
			case <-ctx.Done():
				close(added)
				return
			}
		}
	}()

	return &Adder{
		adder: a,
		out:   outChan,
		added: added,
		wrap:  wrap,
		pin:   pin,
	}, nil
}

func pathFromHash(hash string) string {
	return fmt.Sprintf("/%s/%s", FilestoreType, hash)
}

// AddFile adds a file to the top level IPFS Node
func (fst *Filestore) AddFile(file qfs.File, pin bool) (hash string, err error) {
	node := fst.Node()
	ctx := context.Background()

	fileAdder, err := coreunix.NewAdder(ctx, node.Pinning, node.Blockstore, node.DAG)
	fileAdder.Pin = pin
	// fileAdder.Wrap = file.IsDirectory()
	if err != nil {
		err = fmt.Errorf("error allocating adder: %s", err.Error())
		return
	}

	// wrap in a folder if top level is a file
	if !file.IsDirectory() {
		file = qfs.NewMemdir("/", file)
	}

	errChan := make(chan error, 0)
	outChan := make(chan interface{}, 9)

	fileAdder.Out = outChan

	go func() {
		defer close(outChan)
		for {
			file, err := file.NextFile()
			if err == io.EOF {
				// Finished the list of files.
				break
			} else if err != nil {
				errChan <- err
				return
			}
			if err := fileAdder.AddFile(wrapFile{file}); err != nil {
				errChan <- err
				return
			}
		}
		if _, err = fileAdder.Finalize(); err != nil {
			errChan <- fmt.Errorf("error finalizing file adder: %s", err.Error())
			return
		}
		errChan <- nil
		// node, err := fileAdder.CurRootNode()
		// if err != nil {
		// 	errChan <- fmt.Errorf("error finding root node: %s", err.Error())
		// 	return
		// }
		// if err = fileAdder.PinRoot(); err != nil {
		// 	errChan <- fmt.Errorf("error pinning file root: %s", err.Error())
		// 	return
		// }
		// errChan <- nil
	}()

	for {
		select {
		case out, ok := <-outChan:
			if !ok {
				return
			}
			output := out.(*coreunix.AddEvent)
			if len(output.Hash) > 0 {
				hash = output.Hash
				// return
			}
		case err := <-errChan:
			return hash, err
		}

	}
}

func (fst *Filestore) Pin(ctx context.Context, cid string, recursive bool) error {
	return fst.capi.Pin().Add(ctx, path.New(cid))
}

func (fst *Filestore) Unpin(ctx context.Context, cid string, recursive bool) error {
	return fst.capi.Pin().Rm(ctx, path.New(cid))
}

// PinsetDifference returns a map of "Recursive"-pinned hashes that are not in
// the given set of hash keys. The returned set is a list of all data
func (fst *Filestore) PinsetDifference(ctx context.Context, set map[string]struct{}) (<-chan string, error) {
	resCh := make(chan string, 10)
	res, err := fst.capi.Pin().Ls(ctx, func(o *caopts.PinLsSettings) error {
		o.Type = "recursive"
		return nil
	})
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(resCh)
	LOOP:
		for {
			select {
			case p, ok := <-res:
				if !ok {
					break LOOP
				}

				str := p.Path().String()
				if _, ok := set[str]; !ok {
					// send on channel if path is not in set
					resCh <- str
				}
			case <-ctx.Done():
				log.Debug(ctx.Err())
				break LOOP
			}
		}
	}()

	return resCh, nil
}

type wrapFile struct {
	qfs.File
}

func (w wrapFile) NextFile() (files.File, error) {
	next, err := w.File.NextFile()
	if err != nil {
		return nil, err
	}
	return wrapFile{next}, nil
}

func (w wrapFile) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("wrapFile doesn't support seeking")
}

func (w wrapFile) Size() (int64, error) {
	return 0, fmt.Errorf("wrapFile doesn't support Size")
}

type ipfsFile struct {
	path string
	r    io.ReadCloser
}

var _ qfs.File = (*ipfsFile)(nil)

// Read proxies to the response body reader
func (f ipfsFile) Read(p []byte) (int, error) {
	return f.r.Read(p)
}

// Close proxies to the response body reader
func (f ipfsFile) Close() error {
	return f.r.Close()
}

// IsDirectory satisfies the qfs.File interface
func (f ipfsFile) IsDirectory() bool {
	return false
}

// NextFile satisfies the qfs.File interface
func (f ipfsFile) NextFile() (qfs.File, error) {
	return nil, qfs.ErrNotDirectory
}

// FileName returns a filename associated with this file
func (f ipfsFile) FileName() string {
	return filepath.Base(f.path)
}

// FullPath returns the full path used when adding this file
func (f ipfsFile) FullPath() string {
	return f.path
}

// MediaType maps an ipfs CID to a media type. Media types are not yet
// implemented for ipfs files
// TODO (b5) - finish
func (f ipfsFile) MediaType() string {
	return ""
}

// ModTime gets the last time of modification. ipfs files are immutable
// and will always have a ModTime of zero
func (f ipfsFile) ModTime() time.Time {
	return time.Time{}
}
