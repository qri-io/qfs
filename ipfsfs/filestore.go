package ipfsfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"time"

	// Note coreunix is forked form github.com/ipfs/go-ipfs/core/coreunix
	// we need coreunix.Adder.addFile to be exported to get access to dags while
	// they're being created. We should be able to remove this with refactoring &
	// moving toward coreapi.coreUnix().Add() with properly-configured options,
	// but I'd like a test before we do that. We may also want to consider switching
	// Qri to writing IPLD. Lots to think about.
	coreunix "github.com/qri-io/qfs/ipfsfs/coreunix"
	files "github.com/qri-io/qfs/ipfsfs/go-ipfs-files"
	"github.com/qri-io/value"

	"github.com/ipfs/go-cid"
	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/qri-io/qfs"
	cafs "github.com/qri-io/qfs/cafs"
)

var log = logging.Logger("cafs/ipfs")

const prefix = "ipfs"

// Filestore implements the qfs.Filesystem interface backed by an IPFS node
type Filestore struct {
	cfg  *StoreCfg
	node *core.IpfsNode
	capi coreiface.CoreAPI
}

// NewFilestore creates a filestore with optional configuration
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

// PathPrefix indicates this store works with files of "ipfs" kind
func (fst Filestore) PathPrefix() string {
	return prefix
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

// Online returns true if this store is connected to a peer-2-peer network
func (fst *Filestore) Online() bool {
	return fst.node.IsOnline
}

// GoOnline connects to an IPFS peer-2-peer network
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

// Has checks for the existence of a path
func (fst *Filestore) Has(ctx context.Context, key string) (exists bool, err error) {
	id, err := cid.Parse(key)
	if err != nil {
		return false, err
	}
	return fst.node.Blockstore.Has(id)
}

// Get a key
func (fst *Filestore) Get(ctx context.Context, key string) (qfs.File, error) {
	return fst.getKey(ctx, key)
}

// Fetch implements the fetcher interface, fetching and pinning a key from the
// connected IPFS network
//
// Deprecated: use Get a combination of Has, Get, and a connected node instead
func (fst *Filestore) Fetch(ctx context.Context, source cafs.Source, key string) (qfs.File, error) {
	return fst.getKey(ctx, key)
}

// Put adds a file and pins
func (fst *Filestore) Put(ctx context.Context, file qfs.File) (key string, err error) {
	hash, err := fst.AddFile(ctx, file, true)
	if err != nil {
		log.Infof("error adding bytes: %s", err.Error())
		return
	}
	return pathFromHash(hash), nil
}

// Delete removes & unpins a path
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
//
// Deprecated: use Put instead
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
func (fst *Filestore) AddFile(ctx context.Context, file qfs.File, pin bool) (path string, err error) {
	var adder ipld.NodeAdder = fst.capi.Dag()
	if pin {
		adder = fst.capi.Dag().Pinning()
	}
	b := ipld.NewBatch(ctx, adder)

	// cbor data buffer
	buf := &bytes.Buffer{}

	// TODO (b5) - construct cbor tree, write to bytes this is a placeholder {"hello":"world"} for now
	buf.Write([]byte{0xA1, 0x65, 0x68, 0x65, 0x6C, 0x6C, 0x6F, 0x65, 0x77, 0x6F, 0x72, 0x6C, 0x64})

	// providing math.MaxUint64 means "use the default multihash type", which is
	// sha256 for ipld cbor. using the default type keeps us synced with the ipld
	// ecosystem
	// passing -1 as a multihash length again indicates "use default length"
	nd, err := ipldcbor.Decode(buf.Bytes(), math.MaxUint64, -1)
	if err != nil {
		return "", err
	}

	b.Add(ctx, nd)

	path = nd.Cid().String()

	if err = b.Commit(); err != nil {
		return path, err
	}

	return path, err
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

// Value returns the value of this file
func (f ipfsFile) Value() value.Value {
	return f
}
