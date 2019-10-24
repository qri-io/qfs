package ipfs_filestore

import (
	"context"
	"fmt"
	"io"

	// Note coreunix is forked form github.com/ipfs/go-ipfs/core/coreunix
	// we need coreunix.Adder.addFile to be exported to get access to dags while
	// they're being created. We should be able to remove this with refactoring &
	// moving toward coreapi.coreUnix().Add() with properly-configured options,
	// but I'd like a test before we do that. We may also want to consider switching
	// Qri to writing IPLD. Lots to think about.
	coreunix "github.com/qri-io/qfs/cafs/ipfs/coreunix"

	"github.com/ipfs/go-cid"
	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/qri-io/qfs"
	cafs "github.com/qri-io/qfs/cafs"
	files "github.com/qri-io/qfs/cafs/ipfs/go-ipfs-files"
)

var log = logging.Logger("cafs/ipfs")

const prefix = "ipfs"

type Filestore struct {
	cfg  *StoreCfg
	node *core.IpfsNode
	capi coreiface.CoreAPI
}

func (fst Filestore) PathPrefix() string {
	return prefix
}

func NewFilestore(config ...Option) (*Filestore, error) {
	cfg := DefaultConfig()
	for _, option := range config {
		option(cfg)
	}

	if cfg.Node != nil {
		capi, err := coreapi.NewCoreAPI(cfg.Node)
		if err != nil {
			return nil, err
		}

		return &Filestore{
			cfg:  cfg,
			node: cfg.Node,
			capi: capi,
		}, nil
	}

	if err := cfg.InitRepo(cfg.Ctx); err != nil {
		return nil, err
	}

	node, err := core.NewNode(cfg.Ctx, &cfg.BuildCfg)
	if err != nil {
		return nil, fmt.Errorf("error creating ipfs node: %s", err.Error())
	}

	capi, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return nil, err
	}

	return &Filestore{
		cfg:  cfg,
		node: node,
		capi: capi,
	}, nil
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

	if rdr, ok := node.(io.Reader); ok {
		return qfs.NewMemfileReader(key, rdr), nil
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
	return fmt.Sprintf("/%s/%s", prefix, hash)
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
