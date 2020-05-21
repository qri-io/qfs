package ipfs_filestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs/core"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/mitchellh/mapstructure"
)

// StoreCfg configures the datastore
type StoreCfg struct {
	// embed options for creating a node
	core.BuildCfg
	// optionally just supply a node. will override everything
	Node *core.IpfsNode
	// path to a local filesystem fs repo
	FsRepoPath string
	// operating context
	Ctx context.Context
	// EnableAPI
	EnableAPI bool
	// APIAddr is an ipfs http api address, used as a fallback if we cannot
	// config an ipfs filesystem. The filesystem will instead be a `ipfs_http`
	// filesystem.
	APIAddr string
}

func mapToConfig(cfgmap map[string]interface{}) (*StoreCfg, error) {
	if cfgmap == nil {
		return DefaultConfig(""), nil
	}
	cfg := &StoreCfg{}
	if err := mapstructure.Decode(cfgmap, cfg); err != nil {
		return nil, err
	}
	if cfg.Ctx == nil {
		cfg.Ctx = context.Background()
	}
	return cfg, nil
}

// DefaultConfig results in a local node that
// attempts to draw from the default ipfs filesotre location
func DefaultConfig(path string) *StoreCfg {
	return &StoreCfg{
		BuildCfg: core.BuildCfg{
			Online: false,
		},
		FsRepoPath: path,
		Ctx:        context.Background(),
	}
}

// Option is a function that adjusts the store configuration
type Option func(o *StoreCfg)

// OptEnablePubSub configures ipfs to use the experimental pubsub store
func OptEnablePubSub(o *StoreCfg) {
	o.BuildCfg.ExtraOpts = map[string]bool{
		"pubsub": true,
	}
}

// OptsFromMap detects options from a map based on special keywords
func OptsFromMap(opts map[string]interface{}) Option {
	return func(o *StoreCfg) {
		if opts == nil {
			return
		}

		if api, ok := opts["api"].(bool); ok {
			o.EnableAPI = api
		}

		if ps, ok := opts["pubsub"].(bool); ok {
			if o.BuildCfg.ExtraOpts == nil {
				o.BuildCfg.ExtraOpts = map[string]bool{}
			}
			o.BuildCfg.ExtraOpts["pubsub"] = ps
		}

	}
}

func (cfg *StoreCfg) InitRepo(ctx context.Context) error {
	if cfg.NilRepo {
		return nil
	}
	if cfg.Repo != nil {
		return nil
	}
	if cfg.FsRepoPath != "" {
		if daemonLocked, err := fsrepo.LockedByOtherProcess(cfg.FsRepoPath); err != nil {
			return err
		} else if daemonLocked {
			return errRepoLock
		}
		localRepo, err := fsrepo.Open(cfg.FsRepoPath)
		if err != nil {
			if err == fsrepo.ErrNeedMigration {
				return ErrNeedMigration
			}
			return fmt.Errorf("error opening local filestore ipfs repository: %w", err)
		}
		go func() {
			<-ctx.Done()
			localRepo.Close()
		}()
		cfg.Repo = localRepo
	}
	return nil
}

// MoveIPFSRepoOntoPath moves the ipfs repo from wherever it is,
// indicated by the store config, to live on the given path
// this changes the path in the given config struct
func MoveIPFSRepoOntoPath(o *StoreCfg, path string) error {
	if path == "" {
		return fmt.Errorf("need a path onto which the ipfs repo should be moved")
	}

	newIPFSPath := filepath.Join(path, filepath.Base(o.FsRepoPath))

	if err := os.Rename(o.FsRepoPath, newIPFSPath); err != nil {
		return err
	}
	o.FsRepoPath = newIPFSPath
	return nil
}
