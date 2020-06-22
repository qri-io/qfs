package qipfs_http

import (
	"context"
	"fmt"
	"io"
	"net/http"

	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/mitchellh/mapstructure"

	httpapi "github.com/ipfs/go-ipfs-http-client"
	qfs "github.com/qri-io/qfs"
	cafs "github.com/qri-io/qfs/cafs"
	files "github.com/qri-io/qfs/qipfs/go-ipfs-files"
)

var log = logging.Logger("cafs/ipfs_http")

type Filestore struct {
	capi coreiface.CoreAPI
}

// FSConfig adjusts the behaviour of an FS instance
type FSConfig struct {
	URL string // url to the ipfs api
}

// if no cfgMap is given, return the default config
func mapToConfig(cfgMap map[string]interface{}) (*FSConfig, error) {
	if cfgMap == nil {
		return nil, fmt.Errorf("config with ipfs api url required for ipfs_http")
	}
	cfg := &FSConfig{}
	if err := mapstructure.Decode(cfgMap, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// NewFilesystem creates a new ipfs http path resolver
// from a config map with no options
func NewFilesystem(cfgMap map[string]interface{}) (cafs.Filestore, error) {
	cfg, err := mapToConfig(cfgMap)
	if err != nil {
		return nil, err
	}
	cli, err := httpapi.NewURLApiWithClient(cfg.URL, http.DefaultClient)
	if err != nil {
		return nil, err
	}

	return &Filestore{
		capi: cli,
	}, nil
}

func (fst *Filestore) IPFSCoreAPI() coreiface.CoreAPI {
	return fst.capi
}

// FilestoreType uniquely identifies this filestore
const FilestoreType = "ipfs"

// Type distinguishes this filesystem from others by a unique string prefix
func (fst Filestore) Type() string {
	return FilestoreType
}

// Online always returns true
// TODO (b5): the answer to this is more nuanced. The IPFS api may be available
// but not connected to p2p
func (fst *Filestore) Online() bool {
	return true
}

func (fst *Filestore) Has(ctx context.Context, key string) (exists bool, err error) {
	return false, fmt.Errorf("ipfs_http hasn't implemented has yet")
	// // TODO (b5) - we should be scrutinizing the error that's returned here:
	// if _, err = fst.node.Resolver.ResolvePath(fst.node.Context(), putil.Path(key)); err != nil {
	// 	return false, nil
	// }

	// return true, nil
}

func (fst *Filestore) Get(ctx context.Context, key string) (qfs.File, error) {
	return fst.getKey(ctx, key)
}

func (fst *Filestore) Fetch(ctx context.Context, source cafs.Source, key string) (qfs.File, error) {
	return fst.getKey(ctx, key)
}

func (fst *Filestore) Put(ctx context.Context, file qfs.File) (key string, err error) {
	return "", fmt.Errorf("ipfs_http cannot put")
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

func (fst *Filestore) NewAdder(pin, wrap bool) (cafs.Adder, error) {
	return nil, fmt.Errorf("ipfs_http does not support adders")
}

func pathFromHash(hash string) string {
	return fmt.Sprintf("/%s/%s", FilestoreType, hash)
}

// AddFile adds a file to the top level IPFS Node
func (fst *Filestore) AddFile(ctx context.Context, file qfs.File, pin bool) (hash string, err error) {
	return "", fmt.Errorf("ipfs_http doesn't support adding")
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
